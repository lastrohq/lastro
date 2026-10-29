package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/domain"
)

func TestTodaRotaTemEscopo(t *testing.T) {
	spec, _ := carregarContrato(t)
	for caminho, item := range spec.Paths.Map() {
		for metodo := range item.Operations() {
			rota := metodo + " " + caminho
			if _, ok := escopoDaRota[rota]; !ok && !rotasPublicas[rota] {
				t.Errorf("a rota %q não tem escopo em escopoDaRota nem é pública", rota)
			}
		}
	}
}

func TestSemTokenOuTokenRuimE401(t *testing.T) {
	a := novaAPI(t)
	var p gen.Problema
	for _, auth := range []string{"", "Basic abc", "Bearer lst_invalido", "Bearer " + a.token[:len(a.token)-1] + "x"} {
		st := a.chamar(http.MethodGet, "/cob/"+txid, "", map[string]string{"Authorization": auth}, &p)
		if st != http.StatusUnauthorized || p.Detail != nil {
			t.Errorf("Authorization %q: status %d, detail %v", auth, st, p.Detail)
		}
	}
}

func TestHealthNaoExigeToken(t *testing.T) {
	a := novaAPI(t)
	if st := a.chamar(http.MethodGet, "/health", "", map[string]string{"Authorization": ""}, nil); st != http.StatusOK {
		t.Fatalf("status = %d", st)
	}
}

func TestEscopoFaltandoE403(t *testing.T) {
	a := novaAPI(t)
	soLeitura := a.novoToken("", domain.EscopoCobRead)
	var p gen.Problema
	st := a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, map[string]string{"Authorization": "Bearer " + soLeitura}, &p)
	if st != http.StatusForbidden || p.Detail == nil || !strings.Contains(*p.Detail, "cob:write") {
		t.Fatalf("status %d, %+v", st, p)
	}
	if st := a.chamar(http.MethodGet, "/lastro/v1/contas", "", map[string]string{"Authorization": "Bearer " + soLeitura}, &p); st != http.StatusForbidden {
		t.Fatalf("contas sem conta:admin: %d", st)
	}
}

func TestChaveRevogadaDeixaDeFuncionarNaHora(t *testing.T) {
	a := novaAPI(t)
	k, token, _ := a.keys.Criar(context.Background(), "x", []domain.Escopo{domain.EscopoCobRead}, "")
	h := map[string]string{"Authorization": "Bearer " + token}
	if st := a.chamar(http.MethodGet, "/cob/"+txid, "", h, nil); st != http.StatusNotFound {
		t.Fatalf("antes de revogar: %d", st)
	}
	_ = a.keys.Revogar(context.Background(), k.ID)
	if st := a.chamar(http.MethodGet, "/cob/"+txid, "", h, nil); st != http.StatusUnauthorized {
		t.Fatalf("depois de revogar: %d", st)
	}
}

func TestChaveDeUmaContaNaoEnxergaOutra(t *testing.T) {
	a := novaAPI(t)
	_ = a.store.Contas().Criar(context.Background(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true})
	daOutra := map[string]string{"Authorization": "Bearer " + a.novoToken("outra", domain.EscopoCobRead, domain.EscopoCobWrite)}

	a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil) // criada pela chave geral na conta "loja"
	if st := a.chamar(http.MethodGet, "/cob/"+txid, "", daOutra, nil); st != http.StatusNotFound {
		t.Fatalf("outra conta enxergou: %d", st)
	}
	if st := a.chamar(http.MethodPut, "/cob/outra00000000000000000000001", corpoCob, daOutra, nil); st != http.StatusBadRequest {
		t.Fatalf("outra conta cobrou na chave da loja: %d", st)
	}
}

func TestContasPelaAPI(t *testing.T) {
	a := novaAPI(t)
	var c gen.Conta
	corpo := `{"id":"nova","nome":"Nova","psp":"sandbox","chave":"nova@example.com","ambiente":"homologacao",
	           "credenciais":{"clientId":"id","clientSecret":"segredo-que-nao-volta","certificado":"UEVNLi4u"}}`
	rec := a.chamarBruto(http.MethodPost, "/lastro/v1/contas", corpo, nil)
	if rec.Code != http.StatusCreated || strings.Contains(rec.Body.String(), "segredo-que-nao-volta") {
		t.Fatalf("criar: %d %s", rec.Code, rec.Body)
	}
	var lista struct{ Contas []gen.Conta }
	if st := a.chamar(http.MethodGet, "/lastro/v1/contas", "", nil, &lista); st != http.StatusOK || len(lista.Contas) != 2 {
		t.Fatalf("listar: %d %+v", st, lista)
	}
	if st := a.chamar(http.MethodPost, "/lastro/v1/contas/nova/desativar", "", nil, &c); st != http.StatusOK || c.Ativa {
		t.Fatalf("desativar: %d %+v", st, c)
	}
	var p gen.Problema
	if st := a.chamar(http.MethodGet, "/lastro/v1/contas/nada", "", nil, &p); st != http.StatusNotFound {
		t.Fatalf("inexistente: %d", st)
	}
	if st := a.chamar(http.MethodPost, "/lastro/v1/contas", strings.Replace(corpo, `"psp":"sandbox"`, `"psp":"itau"`, 1), nil, &p); st != http.StatusBadRequest {
		t.Fatalf("psp desconhecido: %d", st)
	}
	if st := a.chamar(http.MethodPost, "/lastro/v1/contas", corpo, nil, &p); st != http.StatusConflict {
		t.Fatalf("duplicada: %d", st)
	}
}
