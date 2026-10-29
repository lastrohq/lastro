package httpapi

import (
	"net/http"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// pixPago cria uma cob de R$ 123,45 pela API, deixa o sandbox pagá-la e grava
// o Pix como o webhook ou a conciliação gravariam. Devolve o endToEndId.
func (a *api) pixPago() string {
	a.t.Helper()
	if st := a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil); st != http.StatusCreated {
		a.t.Fatalf("criar cob: %d", st)
	}
	a.clock.Avancar(sandbox.AtrasoPadrao)
	conta, err := a.store.Contas().Buscar(a.t.Context(), "loja")
	if err != nil {
		a.t.Fatal(err)
	}
	pix, err := a.sandbox.ListarPix(a.t.Context(), conta, ports.Janela{Fim: a.clock.Now().Add(1)})
	if err != nil || len(pix) != 1 {
		a.t.Fatalf("ListarPix = %+v, %v", pix, err)
	}
	if _, err := a.store.Pix().Registrar(a.t.Context(), pix[0]); err != nil {
		a.t.Fatal(err)
	}
	return pix[0].EndToEndID
}

func caminhoDevolucao(e2e, id string) string { return "/pix/" + e2e + "/devolucao/" + id }

func TestSolicitarEConsultarDevolucao(t *testing.T) {
	a := novaAPI(t)
	e2e := a.pixPago()
	var d gen.Devolucao
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev1"), `{"valor":"23.45","descricao":"Item com defeito"}`, nil, &d); st != http.StatusCreated {
		t.Fatalf("solicitar: %d", st)
	}
	if d.Id != "dev1" || d.Valor != "23.45" || d.Natureza != "ORIGINAL" || d.Status != "DEVOLVIDO" || deref(d.Descricao) != "Item com defeito" ||
		deref(d.RtrId) == "" || !d.Horario.Solicitacao.Equal(a.clock.Now()) || d.Horario.Liquidacao == nil {
		t.Fatalf("devolução = %+v", d)
	}
	var lida gen.Devolucao
	if st := a.chamar(http.MethodGet, caminhoDevolucao(e2e, "dev1"), "", nil, &lida); st != http.StatusOK || lida.Id != "dev1" || deref(lida.RtrId) != deref(d.RtrId) {
		t.Fatalf("consultar: %d %+v", st, lida)
	}
	var repetida gen.Devolucao
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev1"), `{"valor":"23.45"}`, nil, &repetida); st != http.StatusCreated || deref(repetida.RtrId) != deref(d.RtrId) {
		t.Fatalf("repetir o par: %d %+v", st, repetida)
	}
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev2"), `{"valor":"100.00","natureza":"ORIGINAL"}`, nil, nil); st != http.StatusCreated {
		t.Fatalf("o resto do saldo: %d", st)
	}
}

func TestSolicitarDevolucaoRecusa(t *testing.T) {
	a := novaAPI(t)
	e2e := a.pixPago()
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev1"), `{"valor":"10.00"}`, nil, nil); st != http.StatusCreated {
		t.Fatalf("primeira: %d", st)
	}
	casos := []struct {
		nome, caminho, corpo string
		status               int
		tipo                 string
	}{
		{"mesmo par com outro valor", caminhoDevolucao(e2e, "dev1"), `{"valor":"11.00"}`, http.StatusConflict, "JaExiste"},
		{"passa do saldo", caminhoDevolucao(e2e, "dev2"), `{"valor":"113.46"}`, http.StatusBadRequest, "RequisicaoInvalida"},
		{"natureza retirada", caminhoDevolucao(e2e, "dev2"), `{"valor":"1.00","natureza":"RETIRADA"}`, http.StatusBadRequest, "RequisicaoInvalida"},
		{"valor fora do formato", caminhoDevolucao(e2e, "dev2"), `{"valor":"1"}`, http.StatusBadRequest, "RequisicaoInvalida"},
		{"pix desconhecido", caminhoDevolucao("E00000000202610291200naoexiste01", "dev2"), `{"valor":"1.00"}`, http.StatusNotFound, "NaoEncontrado"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var p gen.Problema
			if st := a.chamar(http.MethodPut, c.caminho, c.corpo, nil, &p); st != c.status || p.Type != tipoErro(c.tipo) {
				t.Fatalf("status %d tipo %s, quer %d %s (%s)", st, p.Type, c.status, c.tipo, deref(p.Detail))
			}
		})
	}
}

func TestDevolucaoRespeitaEscopoEConta(t *testing.T) {
	a := novaAPI(t)
	e2e := a.pixPago()
	h := func(token string) map[string]string { return map[string]string{"Authorization": "Bearer " + token} }

	soLeitura := h(a.novoToken("loja", domain.EscopoPixRead))
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev1"), `{"valor":"1.00"}`, soLeitura, nil); st != http.StatusForbidden {
		t.Fatalf("sem devolucao:write: %d", st)
	}
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev1"), `{"valor":"1.00"}`, h(a.novoToken("loja", domain.EscopoDevolucaoWrite)), nil); st != http.StatusCreated {
		t.Fatalf("com devolucao:write da conta: %d", st)
	}
	if st := a.chamar(http.MethodGet, caminhoDevolucao(e2e, "dev1"), "", soLeitura, nil); st != http.StatusOK {
		t.Fatalf("consultar com pix:read: %d", st)
	}
	if err := a.store.Contas().Criar(a.t.Context(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	daOutra := h(a.novoToken("outra", domain.EscopoPixRead, domain.EscopoDevolucaoWrite))
	if st := a.chamar(http.MethodGet, caminhoDevolucao(e2e, "dev1"), "", daOutra, nil); st != http.StatusNotFound {
		t.Fatalf("consultar de outra conta: %d", st)
	}
	if st := a.chamar(http.MethodPut, caminhoDevolucao(e2e, "dev2"), `{"valor":"1.00"}`, daOutra, nil); st != http.StatusNotFound {
		t.Fatalf("devolver Pix de outra conta: %d", st)
	}
}
