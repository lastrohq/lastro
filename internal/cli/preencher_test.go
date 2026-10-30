package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
)

// TestServePreencheContaAntigaEAvisaOVencimento simula uma conta do v0.2.0 (o
// certificado só no blob cifrado, já vencido): ao subir, o serve preenche os
// dados públicos e o aviso certificado.expirando chega ao destino (#79).
func TestServePreencheContaAntigaEAvisaOVencimento(t *testing.T) {
	recebidos := make(chan string, 4)
	destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		recebidos <- string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destino.Close()

	env := envComBanco(t)
	env["VINTEM_ADDR"], env["VINTEM_CONCILIACAO_INTERVALO"], env["VINTEM_WEBHOOK_INTERVALO"] = "127.0.0.1:0", "0", "50ms"
	amb, _ := ambiente(t, env) // relógio em 2026-10-29
	ctx := context.Background()

	// A conta entra como o v0.2.0 gravava: direto no store, sem ler o certificado.
	velho := certificadotest.Gerar(t, "loja-antiga", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	if err := contas.Store.Contas().Criar(ctx, domain.Conta{ID: "antiga", Nome: "Antiga", PSP: "sandbox", ChavePix: "antiga@example.com", Ambiente: domain.Producao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	plano, _ := json.Marshal(domain.Credenciais{ClientID: "x", Certificado: velho.P12, SenhaCertificado: certificadotest.Senha}) //nolint:gosec // G117: serializado só para ser cifrado na linha seguinte
	cifrado, _ := contas.Vault.Cifrar(ctx, plano)
	if err := contas.Store.Contas().SalvarCredenciais(ctx, "antiga", cifrado); err != nil {
		t.Fatal(err)
	}
	fechar()
	if err := Run(ctx, []string{"destino", "criar", "--url", destino.URL}, amb); err != nil {
		t.Fatal(err)
	}

	servindo, parar := context.WithCancel(ctx)
	feito := make(chan error, 1)
	go func() { feito <- Run(servindo, []string{"serve"}, amb) }()
	defer func() {
		parar()
		if err := <-feito; err != nil {
			t.Errorf("serve: %v", err)
		}
	}()

	select {
	case corpo := <-recebidos:
		var ev struct {
			Type string               `json:"type"`
			Data app.DadosCertificado `json:"data"`
		}
		_ = json.Unmarshal([]byte(corpo), &ev)
		if ev.Type != "certificado.expirando" || ev.Data.ContaID != "antiga" || !ev.Data.Vencido || !strings.Contains(ev.Data.Sujeito, "loja-antiga") {
			t.Fatalf("evento = %s", corpo)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o aviso da conta antiga não chegou em 5 s")
	}
}
