package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/routers"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/vault"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

type api struct {
	h     http.Handler
	r     routers.Router
	clock *fake.Clock
	store *fake.Store
	keys  app.APIKeys
	// token é a API key usada por padrão nas chamadas (todos os escopos, sem restrição).
	token string
	t     *testing.T
}

func novaAPI(t *testing.T) *api {
	t.Helper()
	clock := fake.NewClock(time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC))
	store := fake.NewStore()
	err := store.Contas().Criar(context.Background(), domain.Conta{ID: "loja", Nome: "Loja", PSP: "sandbox", ChavePix: "loja@example.com", Ambiente: domain.Homologacao, Ativa: true})
	if err != nil {
		t.Fatal(err)
	}
	v, err := vault.New(vault.NovaChave())
	if err != nil {
		t.Fatal(err)
	}
	registry := psp.NewRegistry(sandbox.New(clock))
	keys := app.APIKeys{Store: store, Clock: clock}
	h := NewRouter(Opcoes{
		Versao:    "v-test",
		Cobrancas: app.Cobrancas{Store: store, PSPs: registry, Clock: clock},
		Contas:    app.Contas{Store: store, Vault: v, PSPs: registry},
		APIKeys:   keys,
		Clock:     clock,
	})
	_, r := carregarContrato(t)
	_, token, err := keys.Criar(context.Background(), "testes", domain.TodosEscopos, "")
	if err != nil {
		t.Fatal(err)
	}
	return &api{h: h, r: r, clock: clock, store: store, keys: keys, token: token, t: t}
}

// novoToken cria uma API key com os escopos e a conta dados.
func (a *api) novoToken(conta string, escopos ...domain.Escopo) string {
	a.t.Helper()
	_, token, err := a.keys.Criar(context.Background(), "teste", escopos, conta)
	if err != nil {
		a.t.Fatal(err)
	}
	return token
}

// chamarBruto faz a requisição validada contra o contrato e devolve o recorder.
func (a *api) chamarBruto(metodo, caminho, corpo string, header map[string]string) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequestWithContext(a.t.Context(), metodo, caminho, strings.NewReader(corpo))
	if corpo != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	return conferirResposta(a.t, a.r, a.h, req)
}

// chamar faz a requisição, valida a resposta contra o contrato e decodifica o corpo em out.
func (a *api) chamar(metodo, caminho, corpo string, header map[string]string, out any) int {
	a.t.Helper()
	rec := a.chamarBruto(metodo, caminho, corpo, header)
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			a.t.Fatalf("corpo não decodifica (%d): %s", rec.Code, rec.Body)
		}
	}
	return rec.Code
}

const txid = "pedido00000000000000000000001"

const corpoCob = `{
  "calendario": {"expiracao": 3600},
  "devedor": {"cpf": "12345678909", "nome": "Fulano de Tal"},
  "valor": {"original": "123.45"},
  "chave": "loja@example.com",
  "solicitacaoPagador": "Pedido 1"
}`

func TestCobFluxoCompletoPelaAPI(t *testing.T) {
	a := novaAPI(t)

	var criada gen.CobGerada
	if st := a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, &criada); st != http.StatusCreated {
		t.Fatalf("PUT status = %d", st)
	}
	if criada.Status != gen.CobStatusATIVA || criada.Valor.Original != "123.45" || criada.Revisao != 0 ||
		criada.Calendario.Expiracao != 3600 || criada.PixCopiaECola == nil || criada.Chave != "loja@example.com" ||
		criada.Devedor == nil || criada.Devedor.Cpf == nil {
		t.Fatalf("criada: %+v", criada)
	}

	var c gen.CobCompleta
	a.chamar(http.MethodGet, "/cob/"+txid, "", nil, &c)
	if c.Status != gen.CobStatusATIVA {
		t.Fatalf("antes do pagamento: %s", c.Status)
	}
	a.clock.Avancar(sandbox.AtrasoPadrao)
	if st := a.chamar(http.MethodGet, "/cob/"+txid, "", nil, &c); st != http.StatusOK || c.Status != gen.CobStatusCONCLUIDA || c.Revisao != 1 {
		t.Fatalf("depois do pagamento: %d %s rev %d", st, c.Status, c.Revisao)
	}
}

func TestCobPostGeraTxid(t *testing.T) {
	a := novaAPI(t)
	var criada gen.CobGerada
	if st := a.chamar(http.MethodPost, "/cob", corpoCob, map[string]string{"Idempotency-Key": "k1"}, &criada); st != http.StatusCreated {
		t.Fatalf("status = %d", st)
	}
	if _, err := domain.ParseTxid(criada.Txid); err != nil {
		t.Fatalf("txid gerado inválido: %v", err)
	}
	var p gen.Problema
	if st := a.chamar(http.MethodPost, "/cob", corpoCob, nil, &p); st != http.StatusBadRequest {
		t.Fatalf("sem Idempotency-Key: status = %d", st)
	}
}

func TestCobErrosNoFormatoDaSpec(t *testing.T) {
	a := novaAPI(t)
	var p gen.Problema

	invalido := strings.Replace(corpoCob, `"123.45"`, `"123,45"`, 1)
	if st := a.chamar(http.MethodPut, "/cob/"+txid, invalido, nil, &p); st != http.StatusBadRequest || p.Violacoes == nil || *(*p.Violacoes)[0].Propriedade != "valor.original" {
		t.Fatalf("valor inválido: %d %+v", st, p)
	}
	if st := a.chamar(http.MethodGet, "/cob/inexistente000000000000000001", "", nil, &p); st != http.StatusNotFound {
		t.Fatalf("inexistente: %d", st)
	}
	a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil)
	if st := a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, &p); st != http.StatusConflict {
		t.Fatalf("txid repetido: %d", st)
	}
	recusado := strings.Replace(corpoCob, `"123.45"`, `"0.66"`, 1)
	if st := a.chamar(http.MethodPut, "/cob/recusado0000000000000000000001", recusado, nil, &p); st != http.StatusBadGateway || !strings.HasSuffix(p.Type, "PSPIndisponivel") {
		t.Fatalf("recusa do PSP: %d %+v", st, p)
	}
	if st := a.chamar(http.MethodGet, "/cobv/"+txid, "", nil, &p); st != http.StatusNotImplemented {
		t.Fatalf("cobv: %d", st)
	}
}

func TestCobPatchRemove(t *testing.T) {
	a := novaAPI(t)
	a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil)
	var rev gen.CobGerada
	if st := a.chamar(http.MethodPatch, "/cob/"+txid, `{"valor": {"original": "99.90"}}`, nil, &rev); st != http.StatusOK || rev.Valor.Original != "99.90" || rev.Revisao != 1 {
		t.Fatalf("revisão: %d %+v", st, rev)
	}
	if st := a.chamar(http.MethodPatch, "/cob/"+txid, `{"status": "REMOVIDA_PELO_USUARIO_RECEBEDOR"}`, nil, &rev); st != http.StatusOK || rev.Status != gen.CobStatusREMOVIDAPELOUSUARIORECEBEDOR {
		t.Fatalf("remoção: %d %+v", st, rev)
	}
	var p gen.Problema
	if st := a.chamar(http.MethodPatch, "/cob/"+txid, `{"valor": {"original": "1.00"}}`, nil, &p); st != http.StatusBadRequest || !strings.HasSuffix(p.Type, "CobOperacaoInvalida") {
		t.Fatalf("revisar removida: %d %+v", st, p)
	}
}

func TestCobListarPaginado(t *testing.T) {
	a := novaAPI(t)
	for i := range 3 {
		a.chamar(http.MethodPut, "/cob/lista0000000000000000000000"+string(rune('a'+i)), corpoCob, nil, nil)
	}
	var lista gen.CobsConsultadas
	st := a.chamar(http.MethodGet, "/cob?inicio=2026-10-29T00:00:00Z&fim=2026-10-30T00:00:00Z&paginacao.itensPorPagina=2", "", nil, &lista)
	if st != http.StatusOK || len(lista.Cobs) != 2 || lista.Parametros.Paginacao.QuantidadeTotalDeItens != 3 || lista.Parametros.Paginacao.QuantidadeDePaginas != 2 {
		t.Fatalf("listar: %d %+v", st, lista.Parametros.Paginacao)
	}
}
