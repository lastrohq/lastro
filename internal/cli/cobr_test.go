package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// TestCobrComRetentativaNoSandbox é o critério da #32, entre processos como
// no binário: cada comando monta um sandbox novo, lendo o store. Uma
// recorrência de R$ 0,55 é aprovada, a cobrança falha no vencimento por falta
// de saldo, a retentativa é paga, e cada tentativa vira um evento.
func TestCobrComRetentativaNoSandbox(t *testing.T) {
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	env := envComBanco(t)
	env["LASTRO_ADDR"], env["LASTRO_CONCILIACAO_INTERVALO"], env["LASTRO_WEBHOOK_INTERVALO"] = addr, "0", "0"
	amb, out := ambiente(t, env)
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	amb.Clock = relogio
	amb.MontarPSPs = func(f Fontes) ports.Registry { return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store))) }
	ctx := context.Background()

	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@example.com"}, amb); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "cobr", "--conta", "loja",
		"--escopo", "rec:write", "--escopo", "rec:read", "--escopo", "cobr:write", "--escopo", "cobr:read"}, amb); err != nil {
		t.Fatal(err)
	}
	token := regexp.MustCompile(`lst_\S+`).FindString(out.String())
	base := "http://" + addr

	// servir sobe um serve, roda as chamadas e para, como um processo que termina.
	servir := func(chamadas func(pedir func(metodo, caminho, idem, corpo string, saida any) int)) {
		t.Helper()
		servindo, parar := context.WithCancel(ctx)
		feito := make(chan error, 1)
		go func() { feito <- Run(servindo, []string{"serve"}, amb) }()
		pedir := func(metodo, caminho, idem, corpo string, saida any) int {
			t.Helper()
			req, _ := http.NewRequestWithContext(ctx, metodo, base+caminho, strings.NewReader(corpo))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			if idem != "" {
				req.Header.Set("Idempotency-Key", idem)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return 0
			}
			defer func() { _ = resp.Body.Close() }()
			b, _ := io.ReadAll(resp.Body)
			if saida != nil {
				_ = json.Unmarshal(b, saida)
			}
			return resp.StatusCode
		}
		for i := 0; pedir(http.MethodGet, "/health", "", "", nil) != http.StatusOK; i++ {
			if i > 100 {
				t.Fatal("o serve não subiu")
			}
			time.Sleep(20 * time.Millisecond)
		}
		chamadas(pedir)
		parar()
		if err := <-feito; err != nil {
			t.Fatalf("serve: %v", err)
		}
	}
	conciliar := func(quer string) {
		t.Helper()
		out.Reset()
		if err := Run(ctx, []string{"conciliar"}, amb); err != nil {
			t.Fatalf("conciliar: %v (%s)", err, out)
		}
		if !strings.Contains(out.String(), quer) {
			t.Fatalf("conciliar: %s; quer %q", out, quer)
		}
	}
	const txid = "cobrcli000000000000000000001"
	var idRec string

	// 1. A recorrência de R$ 0,55 é criada, pedida e aprovada pelo pagador.
	servir(func(pedir func(string, string, string, string, any) int) {
		var rec struct {
			IDRec string `json:"idRec"`
		}
		corpo := `{"vinculo":{"contrato":"c1","devedor":{"cpf":"12345678909","nome":"Fulano"}},"calendario":{"dataInicial":"2026-11-01","periodicidade":"MENSAL"},"valor":{"valorRec":"0.55"},"politicaRetentativa":"PERMITE_3R_7D"}`
		if st := pedir(http.MethodPost, "/rec", "rec-1", corpo, &rec); st != http.StatusCreated {
			t.Fatalf("criar recorrência: %d", st)
		}
		idRec = rec.IDRec
		sol := fmt.Sprintf(`{"idRec":%q,"calendario":{"dataExpiracaoSolicitacao":%q},"destinatario":{"conta":"1","ispbParticipante":"18236120"}}`, idRec, relogio.Now().Add(24*time.Hour).Format(time.RFC3339))
		if st := pedir(http.MethodPost, "/solicrec", "sol-1", sol, nil); st != http.StatusCreated {
			t.Fatalf("solicitar: %d", st)
		}
	})
	relogio.Avancar(sandbox.AtrasoPadrao)
	conciliar("1 recorrências atualizadas")

	// 2. A cobrança de novembro.
	servir(func(pedir func(string, string, string, string, any) int) {
		corpo := fmt.Sprintf(`{"idRec":%q,"calendario":{"dataDeVencimento":"2026-11-05"},"valor":{"original":"0.55"}}`, idRec)
		if st := pedir(http.MethodPut, "/cobr/"+txid, "", corpo, nil); st != http.StatusCreated {
			t.Fatalf("criar cobrança: %d", st)
		}
	})

	// 3. A tentativa do vencimento falha por falta de saldo; nenhum aviso chega,
	// e a conciliação encontra.
	relogio.Avancar(2 * sandbox.AtrasoPadrao)
	conciliar("1 cobranças recorrentes atualizadas")

	// 4. No dia do vencimento, o recebedor pede uma nova data.
	relogio.Avancar(time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC).Sub(relogio.Now()))
	servir(func(pedir func(string, string, string, string, any) int) {
		var c struct {
			Status     string `json:"status"`
			Tentativas []struct {
				Tipo, Status string
			} `json:"tentativas"`
		}
		if st := pedir(http.MethodPost, "/cobr/"+txid+"/retentativa/2026-11-07", "", "", &c); st != http.StatusCreated {
			t.Fatalf("retentativa: %d", st)
		}
		if c.Status != "ATIVA" || len(c.Tentativas) != 2 || c.Tentativas[0].Status != "REJEITADA" || c.Tentativas[1].Tipo != "NTAG" {
			t.Fatalf("depois de pedir a retentativa: %+v", c)
		}
	})

	// 5. A retentativa é paga, e a conciliação fecha a cobrança.
	relogio.Avancar(sandbox.AtrasoPadrao)
	conciliar("1 cobranças recorrentes atualizadas")
	conciliar("0 cobranças recorrentes atualizadas")

	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	c, err := contas.Store.CobrancasRecorrentes().BuscarPorTxid(ctx, txid)
	if err != nil || c.Status != domain.CobrConcluida || len(c.Tentativas) != 2 || c.Tentativas[1].Status != domain.TentativaPaga || c.Tentativas[1].EndToEndID == "" {
		t.Fatalf("no fim: %+v, %v", c, err)
	}
	// Um evento por tentativa: a do vencimento rejeitada, a nova agendada e a nova paga.
	evs, _ := contas.Store.Outbox().NaoDistribuidos(ctx, 100)
	var tentativas []string
	for _, e := range evs {
		if e.Tipo != domain.EventoCobrRTentativa {
			continue
		}
		var d app.DadosCobrancaRecorrente
		_ = json.Unmarshal(e.Dados, &d)
		tentativas = append(tentativas, d.Tentativa.Tipo+" "+d.Tentativa.Status)
	}
	if strings.Join(tentativas, ", ") != "AGND REJEITADA, NTAG AGENDADA, NTAG PAGA" {
		t.Fatalf("eventos de tentativa: %v", tentativas)
	}
}
