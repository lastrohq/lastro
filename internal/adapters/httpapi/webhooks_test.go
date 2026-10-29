package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
)

// configurarWebhook registra a URL da conta loja no sandbox e devolve o caminho dela.
func (a *api) configurarWebhook() string {
	a.t.Helper()
	endereco, err := a.webhooks.Configurar(context.Background(), "loja", "https://lastro.example.com")
	if err != nil {
		a.t.Fatal(err)
	}
	return strings.TrimPrefix(endereco, "https://lastro.example.com")
}

func corpoWebhook(txid, valor string) string {
	return `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","txid":"` + txid + `","valor":"` + valor + `","horario":"2026-10-29T12:00:05Z"}]}`
}

func TestWebhookDoBancoConcluiACobranca(t *testing.T) {
	a := novaAPI(t)
	caminho := a.configurarWebhook()
	a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil)

	// O banco não manda API key, e a spec manda acrescentar /pix à URL.
	semToken := map[string]string{"Authorization": ""}
	if st := a.chamar(http.MethodPost, caminho+"/pix", corpoWebhook(txid, "123.45"), semToken, nil); st != http.StatusOK {
		t.Fatalf("webhook: %d", st)
	}
	if st := a.chamar(http.MethodPost, caminho, corpoWebhook(txid, "123.45"), semToken, nil); st != http.StatusOK {
		t.Fatalf("repetido, sem o sufixo: %d", st)
	}
	var c gen.CobCompleta
	a.chamar(http.MethodGet, "/cob/"+txid, "", nil, &c)
	if c.Status != gen.CobStatusCONCLUIDA || c.Pix == nil || len(*c.Pix) != 1 || (*c.Pix)[0].Valor != "123.45" {
		t.Fatalf("cob depois do webhook: %s %+v", c.Status, c.Pix)
	}
	if n := len(a.store.Eventos()); n != 3 { // cobranca.criada, cobranca.concluida, pix.recebido
		t.Fatalf("%d eventos, quer 3", n)
	}
}

func TestWebhookRecusaTokenErradoECorpoInvalido(t *testing.T) {
	a := novaAPI(t)
	caminho := a.configurarWebhook()
	semToken := map[string]string{"Authorization": ""}
	var p gen.Problema
	errado := caminho[:len(caminho)-1] + "0"
	if strings.HasSuffix(caminho, "0") {
		errado = caminho[:len(caminho)-1] + "1"
	}
	if st := a.chamar(http.MethodPost, errado, corpoWebhook(txid, "1.00"), semToken, &p); st != http.StatusUnauthorized {
		t.Fatalf("token errado: %d", st)
	}
	if st := a.chamar(http.MethodPost, caminho, `{"pix":[{"endToEndId":"curto"}]}`, semToken, &p); st != http.StatusBadRequest || !strings.HasSuffix(p.Type, "WebhookInvalido") {
		t.Fatalf("corpo inválido: %d %+v", st, p)
	}
	// Token fora do formato também é 401: a resposta não revela como o token é.
	if st := a.chamar(http.MethodPost, "/webhooks/pix/loja/naoehex", corpoWebhook(txid, "1.00"), semToken, &p); st != http.StatusUnauthorized {
		t.Fatalf("token fora do formato: %d", st)
	}
}

func TestCorpoAcimaDe1MiBERecusado(t *testing.T) {
	a := novaAPI(t)
	caminho := a.configurarWebhook()
	grande := `{"pix":[],"lixo":"` + strings.Repeat("x", 1<<20) + `"}`
	rec := a.chamarBruto(http.MethodPost, caminho, grande, map[string]string{"Authorization": ""})
	if rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("corpo de 1 MiB+: status %d", rec.Code)
	}
}

// A lista (GET /cob) traz os Pix de cada cobrança paga, como a consulta: na
// spec, os itens são cobranças completas. O painel soma o recebido por aqui.
func TestListaTrazOsPixDaCobrancaPaga(t *testing.T) {
	a := novaAPI(t)
	caminho := a.configurarWebhook()
	a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil)
	a.chamar(http.MethodPut, "/cob/outra000000000000000000000001", corpoCob, nil, nil)
	if st := a.chamar(http.MethodPost, caminho, corpoWebhook(txid, "123.45"), map[string]string{"Authorization": ""}, nil); st != http.StatusOK {
		t.Fatalf("webhook: %d", st)
	}
	var lista gen.CobsConsultadas
	a.chamar(http.MethodGet, "/cob?inicio=2026-10-29T00:00:00Z&fim=2026-10-30T00:00:00Z", "", nil, &lista)
	pix := map[string]int{}
	for _, c := range lista.Cobs {
		if c.Pix != nil {
			pix[c.Txid] = len(*c.Pix)
		}
	}
	if len(lista.Cobs) != 2 || pix[txid] != 1 || pix["outra000000000000000000000001"] != 0 {
		t.Fatalf("pix por cobrança na lista: %v", pix)
	}
}
