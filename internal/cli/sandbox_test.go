package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

// TestSandboxLembraDasCobrancasDepoisDeReiniciar monta o sandbox como no binário
// (com o store), cria uma cobrança, "reinicia" (cada abrir monta um sandbox novo,
// sem memória) e confere que a cobrança é paga e a conciliação a encontra (#64).
func TestSandboxLembraDasCobrancasDepoisDeReiniciar(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	amb.Clock = relogio
	amb.MontarPSPs = func(f Fontes) ports.Registry {
		return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store)))
	}
	ctx := context.Background()
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@example.com"}, amb); err != nil {
		t.Fatal(err)
	}

	// Primeiro processo: cria a cobrança e sai.
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	cobs := app.Cobrancas{Store: contas.Store, PSPs: contas.PSPs, Clock: relogio}
	criada, err := cobs.Criar(ctx, app.PedidoCob{Chave: "loja@example.com", Valor: "25.00"})
	fechar()
	if err != nil {
		t.Fatal(err)
	}

	// Segundo processo, depois do atraso do sandbox: a conciliação encontra o Pix.
	relogio.Avancar(sandbox.AtrasoPadrao + time.Second)
	out.Reset()
	if err := Run(ctx, []string{"conciliar"}, amb); err != nil {
		t.Fatalf("conciliar: %v (%s)", err, out)
	}
	if !strings.Contains(out.String(), "1 Pix recuperados") {
		t.Fatalf("depois do restart: %s", out)
	}

	// Terceiro processo: a cobrança aparece paga.
	contas, fechar, err = abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	lida, err := app.Cobrancas{Store: contas.Store, PSPs: contas.PSPs, Clock: relogio}.Consultar(ctx, string(criada.Txid))
	if err != nil || lida.Status != "CONCLUIDA" {
		t.Fatalf("cobrança depois do restart: %s, %v", lida.Status, err)
	}
}
