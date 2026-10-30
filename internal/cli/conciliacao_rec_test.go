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

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

// TestRecorrenciaAprovadaEntreProcessosPelaConciliacao é o critério da #93:
// cada comando monta um sandbox novo (como processos de verdade), o aviso do
// webhook não tem para onde ir, e a conciliação encontra a aprovação.
func TestRecorrenciaAprovadaEntreProcessosPelaConciliacao(t *testing.T) {
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	env := envComBanco(t)
	env["VINTEM_ADDR"], env["VINTEM_CONCILIACAO_INTERVALO"], env["VINTEM_WEBHOOK_INTERVALO"] = addr, "0", "0"
	amb, out := ambiente(t, env)
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	amb.Clock = relogio
	// Como no binário: um sandbox novo a cada abrir, lendo o store.
	amb.MontarPSPs = func(f Fontes) ports.Registry { return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store))) }
	ctx := context.Background()

	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@example.com"}, amb); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "rec", "--conta", "loja", "--escopo", "rec:write", "--escopo", "rec:read"}, amb); err != nil {
		t.Fatal(err)
	}
	token := regexp.MustCompile(`vtm_\S+`).FindString(out.String())

	// Primeiro processo: o serve cria a recorrência e pede a confirmação.
	servindo, parar := context.WithCancel(ctx)
	feito := make(chan error, 1)
	go func() { feito <- Run(servindo, []string{"serve"}, amb) }()
	base := "http://" + addr
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
	var rec struct {
		IDRec  string `json:"idRec"`
		Status string `json:"status"`
	}
	corpo := `{"vinculo":{"contrato":"c1","devedor":{"cpf":"12345678909","nome":"Fulano"}},"calendario":{"dataInicial":"2026-11-10","periodicidade":"MENSAL"},"valor":{"valorRec":"49.90"},"politicaRetentativa":"NAO_PERMITE"}`
	if st := pedir(http.MethodPost, "/rec", "rec-1", corpo, &rec); st != http.StatusCreated {
		t.Fatalf("criar: %d", st)
	}
	sol := fmt.Sprintf(`{"idRec":%q,"calendario":{"dataExpiracaoSolicitacao":%q},"destinatario":{"conta":"1","ispbParticipante":"18236120"}}`, rec.IDRec, relogio.Now().Add(24*time.Hour).Format(time.RFC3339))
	if st := pedir(http.MethodPost, "/solicrec", "sol-1", sol, nil); st != http.StatusCreated {
		t.Fatalf("solicitar: %d", st)
	}
	parar()
	if err := <-feito; err != nil {
		t.Fatalf("serve: %v", err)
	}

	// O pagador aprova depois do atraso; nenhum aviso chega.
	relogio.Avancar(sandbox.AtrasoPadrao)

	// Outro processo: a conciliação consulta o banco e aplica.
	out.Reset()
	if err := Run(ctx, []string{"conciliar"}, amb); err != nil {
		t.Fatalf("conciliar: %v (%s)", err, out)
	}
	if !strings.Contains(out.String(), "1 recorrências atualizadas") {
		t.Fatalf("conciliar: %s", out)
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	gravada, err := contas.Store.Recorrencias().Buscar(ctx, rec.IDRec)
	if err != nil || gravada.Status != "APROVADA" {
		t.Fatalf("depois da conciliação: %s, %v", gravada.Status, err)
	}
	// Rodar de novo não muda mais nada.
	out.Reset()
	_ = Run(ctx, []string{"conciliar"}, amb)
	if !strings.Contains(out.String(), "0 recorrências atualizadas") {
		t.Fatalf("segunda conciliação: %s", out)
	}
}
