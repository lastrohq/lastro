package psp

import (
	"errors"
	"strings"
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

func TestWebhookPixBCBComDevolucoes(t *testing.T) {
	corpo := `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","valor":"49.90","horario":"2026-10-29T12:00:05-03:00",
	  "devolucoes":[
	    {"id":"dev1","rtrId":"D12345678202610291300abcdefghijk","valor":"10.00","horario":{"solicitacao":"2026-10-29T13:00:00-03:00","liquidacao":"2026-10-29T13:00:02-03:00"},"status":"DEVOLVIDO"},
	    {"id":"dev2","rtrId":"D12345678202610291301abcdefghijk","valor":"5.00","horario":{"solicitacao":"2026-10-29T13:01:00-03:00"},"status":"NAO_REALIZADO","motivo":"Conta encerrada"}]}]}`
	pix, err := WebhookPixBCB(contaTeste, []byte(corpo))
	if err != nil {
		t.Fatal(err)
	}
	ds := pix[0].Devolucoes
	if len(ds) != 2 || ds[0].ID != "dev1" || ds[0].EndToEndID != pix[0].EndToEndID || ds[0].ContaID != "loja" || ds[0].Status != domain.DevolucaoDevolvido ||
		ds[0].Valor != 1000 || ds[0].LiquidadaEm.Hour() != 16 || ds[0].RtrID == "" ||
		ds[1].Status != domain.DevolucaoNaoRealizado || ds[1].Motivo != "Conta encerrada" || !ds[1].LiquidadaEm.IsZero() {
		t.Fatalf("devoluções = %+v", ds)
	}

	ruins := map[string]string{
		"status inventado": `"status":"DEVOLVIDO"`,
		"id com hífen":     `"id":"dev1"`,
		"liquidação ruim":  `"liquidacao":"2026-10-29T13:00:02-03:00"`,
	}
	troca := map[string]string{"status inventado": `"status":"ESTORNADO"`, "id com hífen": `"id":"dev-1"`, "liquidação ruim": `"liquidacao":"29/10"`}
	for nome, de := range ruins {
		if _, err := WebhookPixBCB(contaTeste, []byte(strings.Replace(corpo, de, troca[nome], 1))); !errors.Is(err, domain.ErrPixInvalido) {
			t.Errorf("%s: err = %v", nome, err)
		}
	}
}
