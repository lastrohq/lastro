package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/adapters/psp/efi"
	"github.com/vintemhq/vintem/internal/adapters/psp/efi/efitest"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

// TestContaEfiDePontaAPonta usa o adapter efi de verdade contra o Efí falso: as
// credenciais entram cifradas pelo conta criar e chegam ao adapter pelo
// MontarPSPs, como no binário.
func TestContaEfiDePontaAPonta(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", validadeTeste)
	srv := efitest.Novo(t, par.Certificado, "client-id-efi", "client-secret-efi")
	amb, out := ambiente(t, envComBanco(t))
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	amb.Clock = relogio
	amb.MontarPSPs = func(f Fontes) ports.Registry {
		return psp.NewRegistry(efi.New(f.Credenciais, relogio, efi.ComURLs("https://producao.invalid", srv.URL), efi.ComRaizes(srv.Raizes)))
	}
	ctx := context.Background()

	err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "efi", "--chave", "loja@example.com",
		"--client-id", "client-id-efi", "--client-secret-arquivo", arquivo(t, "secret", []byte("client-secret-efi")),
		"--certificado", arquivo(t, "cert.p12", par.P12), "--senha-certificado-arquivo", arquivo(t, "senha", []byte(certificadotest.Senha))}, amb)
	if err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run(ctx, []string{"conta", "webhook", "loja", "--url-publica", "https://vintem.example.com"}, amb); err != nil {
		t.Fatal(err)
	}
	cadastrada := srv.Webhook("loja@example.com")
	if !strings.HasPrefix(cadastrada, "https://vintem.example.com/webhooks/pix/loja/") || !strings.Contains(out.String(), cadastrada) {
		t.Fatalf("webhook no Efí = %q; saída: %s", cadastrada, out)
	}

	// Um Pix por chave que o Efí não avisou: a conciliação recupera.
	srv.AdicionarPix(1, relogio.Now().Add(-10*time.Minute))
	out.Reset()
	if err := Run(ctx, []string{"conciliar"}, amb); err != nil {
		t.Fatalf("conciliar: %v (%s)", err, out)
	}
	if !strings.Contains(out.String(), "1 Pix recuperados") {
		t.Fatalf("conciliar: %s", out)
	}
	// Rodar de novo não recupera o mesmo Pix (deduplicado pelo endToEndId).
	out.Reset()
	if err := Run(ctx, []string{"conciliar"}, amb); err != nil || !strings.Contains(out.String(), "0 Pix recuperados") {
		t.Fatalf("segunda conciliação: %v (%s)", err, out)
	}
}
