package efi_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/vintemhq/vintem/internal/domain"
)

func TestConfigurarWebhookNaChaveDaContaComSkipMTLS(t *testing.T) {
	c := novo(t)
	url := "https://vintem.example.com/webhooks/pix/loja/segredo"
	if err := c.psp.ConfigurarWebhook(context.Background(), c.conta, url); err != nil {
		t.Fatal(err)
	}
	if got := c.efi.Webhook("loja@example.com"); got != url {
		t.Fatalf("webhook cadastrado = %q; quer %q", got, url)
	}
	if !slices.Contains(c.efi.Chamadas(), "PUT /v2/webhook/loja@example.com") {
		t.Fatalf("chamadas = %v", c.efi.Chamadas())
	}
}

// O exemplo de aviso da documentação do Efí, com um endToEndId de 32 caracteres:
// o da doc ("E1803615022211340s08793XPJ") tem 26 e não passaria na regra do SPI.
const avisoEfi = `{"pix":[{"endToEndId":"E18036150202012211340s08793XPJ12","txid":"fc9a43k6ff384ryP5f41719","chave":"2c3c7441-b91e-4982-3c25-6105581e18ae","valor":"0.01","horario":"2020-12-21T13:40:34.000Z","infoPagador":"pagando o pix"}]}`

func TestInterpretarWebhook(t *testing.T) {
	c := novo(t)
	pix, err := c.psp.InterpretarWebhook(c.conta, []byte(avisoEfi))
	if err != nil || len(pix) != 1 {
		t.Fatalf("aviso da doc: %+v, %v", pix, err)
	}
	if p := pix[0]; p.EndToEndID != "E18036150202012211340s08793XPJ12" || p.Valor != 1 || p.ContaID != "loja" || p.Txid != "fc9a43k6ff384ryP5f41719" {
		t.Fatalf("Pix = %+v", p)
	}

	// Notificação de teste do cadastro (corpo não documentado) ou evento que o Vintém não usa: aceita, sem Pix.
	for _, corpo := range []string{`{}`, `{"evento":"teste_webhook"}`, `{"recs":[]}`} {
		pix, err := c.psp.InterpretarWebhook(c.conta, []byte(corpo))
		if err != nil || len(pix) != 0 {
			t.Fatalf("%s: %+v, %v; quer aceito sem Pix", corpo, pix, err)
		}
	}

	// Com "pix" fora do formato, continua sendo erro: um Pix não some em silêncio.
	for _, corpo := range []string{`{"pix":[]}`, `{"pix":[{"endToEndId":"curto","valor":"1.00"}]}`, `{"pix":"x"}`, `não é json`} {
		if _, err := c.psp.InterpretarWebhook(c.conta, []byte(corpo)); !errors.Is(err, domain.ErrPixInvalido) {
			t.Fatalf("%s: err = %v; quer ErrPixInvalido", corpo, err)
		}
	}
}
