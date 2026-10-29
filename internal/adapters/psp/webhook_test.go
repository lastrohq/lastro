package psp

import (
	"errors"
	"testing"

	"github.com/lastrohq/lastro/internal/domain"
)

var contaTeste = domain.Conta{ID: "loja"}

func TestWebhookPixBCB(t *testing.T) {
	corpo := `{"pix":[
	  {"endToEndId":"E12345678202610291200abcdefghijk","txid":"pedido00000000000000000000001","valor":"49.90",
	   "chave":"loja@example.com","horario":"2026-10-29T12:00:05-03:00","infoPagador":"obrigado"},
	  {"endToEndId":"E12345678202610291201abcdefghijl","valor":"10.00","horario":"2026-10-29T15:01:00Z"}]}`
	pix, err := WebhookPixBCB(contaTeste, []byte(corpo))
	if err != nil {
		t.Fatal(err)
	}
	if len(pix) != 2 || pix[0].Valor != 4990 || pix[0].Txid != "pedido00000000000000000000001" || pix[0].ContaID != "loja" ||
		pix[0].Horario.Hour() != 15 || pix[0].InfoPagador != "obrigado" || pix[1].Txid != "" {
		t.Fatalf("pix = %+v", pix)
	}
}

func TestWebhookPixBCBRecusaLoteComItemInvalido(t *testing.T) {
	casos := map[string]string{
		"não é JSON":   `pix`,
		"lista vazia":  `{"pix":[]}`,
		"sem lista":    `{}`,
		"e2e curto":    `{"pix":[{"endToEndId":"E123","valor":"1.00","horario":"2026-10-29T12:00:00Z"}]}`,
		"valor ruim":   `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","valor":"1,00","horario":"2026-10-29T12:00:00Z"}]}`,
		"horário ruim": `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","valor":"1.00","horario":"29/10/2026"}]}`,
		"valor zero":   `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","valor":"0.00","horario":"2026-10-29T12:00:00Z"}]}`,
	}
	for nome, corpo := range casos {
		if _, err := WebhookPixBCB(contaTeste, []byte(corpo)); !errors.Is(err, domain.ErrPixInvalido) {
			t.Errorf("%s: err = %v", nome, err)
		}
	}
}
