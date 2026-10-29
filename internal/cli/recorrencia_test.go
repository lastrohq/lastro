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
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// TestRecorrenciaAprovadaPeloWebhookNoSandbox é o critério da #31: a
// recorrência vai de CRIADA a APROVADA no sandbox pelo webhook de recorrência.
// Tudo de verdade: serve, API, o sandbox entregando o aviso por HTTP na rota de
// entrada, e a transição gravada. Também rejeição e expiração.
//
// O sandbox é um só nos três comandos porque guarda a URL do webhook em
// memória: entre processos de verdade ela se perde (limite registrado na #93).
func TestRecorrenciaAprovadaPeloWebhookNoSandbox(t *testing.T) {
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
	sb := sandbox.New(relogio, sandbox.ComAtraso(100*time.Millisecond))
	amb.MontarPSPs = func(Fontes) ports.Registry { return psp.NewRegistry(sb) }
	ctx := context.Background()

	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@example.com"}, amb); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "rec", "--conta", "loja", "--escopo", "rec:write", "--escopo", "rec:read"}, amb); err != nil {
		t.Fatal(err)
	}
	token := regexp.MustCompile(`lst_\S+`).FindString(out.String())

	servindo, parar := context.WithCancel(ctx)
	feito := make(chan error, 1)
	go func() { feito <- Run(servindo, []string{"serve"}, amb) }()
	defer func() {
		parar()
		if err := <-feito; err != nil {
			t.Errorf("serve: %v", err)
		}
	}()
	base := "http://" + addr
	for i := 0; ; i++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
			break
		}
		if i > 100 {
			t.Fatal("o serve não subiu")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := Run(ctx, []string{"conta", "webhook", "loja", "--url-publica", base}, amb); err != nil {
		t.Fatal(err)
	}

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
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		if saida != nil {
			_ = json.Unmarshal(b, saida)
		}
		return resp.StatusCode
	}

	for _, c := range []struct {
		valor, status, motivo string
	}{
		{"99.90", "APROVADA", ""},
		{"0.77", "REJEITADA", "AP13"},
		{"0.88", "EXPIRADA", ""},
	} {
		t.Run(c.status, func(t *testing.T) {
			var rec struct {
				IDRec          string `json:"idRec"`
				Status         string `json:"status"`
				MotivoRejeicao string `json:"motivoRejeicao"`
			}
			corpo := fmt.Sprintf(`{"vinculo":{"contrato":"c-%s","devedor":{"cpf":"12345678909","nome":"Fulano"}},"calendario":{"dataInicial":"2026-11-10","periodicidade":"MENSAL"},"valor":{"valorRec":%q},"politicaRetentativa":"NAO_PERMITE"}`, c.status, c.valor)
			if st := pedir(http.MethodPost, "/rec", "rec-"+c.status, corpo, &rec); st != http.StatusCreated || rec.Status != "CRIADA" {
				t.Fatalf("criar: %d %+v", st, rec)
			}
			expira := relogio.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339)
			sol := fmt.Sprintf(`{"idRec":%q,"calendario":{"dataExpiracaoSolicitacao":%q},"destinatario":{"conta":"123456","ispbParticipante":"18236120"}}`, rec.IDRec, expira)
			if st := pedir(http.MethodPost, "/solicrec", "sol-"+c.status, sol, nil); st != http.StatusCreated {
				t.Fatalf("solicitar: %d", st)
			}
			// O sandbox responde depois de 100 ms e avisa pelo webhook.
			prazo := time.Now().Add(5 * time.Second)
			for rec.Status != c.status && time.Now().Before(prazo) {
				time.Sleep(50 * time.Millisecond)
				pedir(http.MethodGet, "/rec/"+rec.IDRec, "", "", &rec)
			}
			if rec.Status != c.status || !strings.HasPrefix(rec.MotivoRejeicao, c.motivo) {
				t.Fatalf("depois do aviso: %s %q; quer %s %q", rec.Status, rec.MotivoRejeicao, c.status, c.motivo)
			}
		})
	}
}
