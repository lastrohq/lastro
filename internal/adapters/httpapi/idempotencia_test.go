package httpapi

import (
	"context"
	"crypto/sha256"
	"net/http"
	"strings"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

func (a *api) totalCobs() int {
	a.t.Helper()
	n, err := a.store.Cobrancas().Contar(context.Background(), ports.FiltroCobrancas{})
	if err != nil {
		a.t.Fatal(err)
	}
	return n
}

func TestIdempotencyKeyRepeteSemCriarDeNovo(t *testing.T) {
	a := novaAPI(t)
	h := map[string]string{"Idempotency-Key": "pedido-42"}

	primeira := a.chamarBruto(http.MethodPost, "/cob", corpoCob, h)
	segunda := a.chamarBruto(http.MethodPost, "/cob", corpoCob, h)
	if primeira.Code != http.StatusCreated || segunda.Code != http.StatusCreated {
		t.Fatalf("status %d e %d", primeira.Code, segunda.Code)
	}
	if segunda.Body.String() != primeira.Body.String() || segunda.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("a repetição não devolveu a resposta original:\n%s\n%s", primeira.Body, segunda.Body)
	}
	if n := a.totalCobs(); n != 1 {
		t.Fatalf("criou %d cobranças, quer 1", n)
	}

	var p gen.Problema
	outra := strings.Replace(corpoCob, "123.45", "999.99", 1)
	if st := a.chamar(http.MethodPost, "/cob", outra, h, &p); st != http.StatusConflict || !strings.HasSuffix(p.Type, "IdempotencyKeyReutilizada") {
		t.Fatalf("mesma chave, outro corpo: %d %+v", st, p)
	}
}

func TestIdempotencyKeyEDeCadaAPIKey(t *testing.T) {
	a := novaAPI(t)
	outroCliente := a.novoToken("", domain.EscopoCobWrite)
	a.chamar(http.MethodPost, "/cob", corpoCob, map[string]string{"Idempotency-Key": "k"}, nil)
	a.chamar(http.MethodPost, "/cob", corpoCob, map[string]string{"Idempotency-Key": "k", "Authorization": "Bearer " + outroCliente}, nil)
	if n := a.totalCobs(); n != 2 {
		t.Fatalf("clientes diferentes com a mesma chave criaram %d cobranças, quer 2", n)
	}
}

func TestIdempotencyKeyNaoGuardaErroDoPSP(t *testing.T) {
	a := novaAPI(t)
	h := map[string]string{"Idempotency-Key": "recusado"}
	recusado := strings.Replace(corpoCob, "123.45", "0.66", 1)
	for range 2 {
		rec := a.chamarBruto(http.MethodPost, "/cob", recusado, h)
		if rec.Code != http.StatusBadGateway || rec.Header().Get("Idempotent-Replayed") != "" {
			t.Fatalf("status %d, replay %q", rec.Code, rec.Header().Get("Idempotent-Replayed"))
		}
	}
}

func TestIdempotencyKeyComRequisicaoEmAndamento(t *testing.T) {
	a := novaAPI(t)
	k, err := a.keys.Autenticar(context.Background(), a.token)
	if err != nil {
		t.Fatal(err)
	}
	// Simula a primeira requisição ainda rodando: a reserva existe, sem resposta.
	hash := sha256.Sum256([]byte("POST /cob\n" + corpoCob))
	idem := app.Idempotencia{Store: a.store, Clock: a.clock}
	if _, err := idem.Iniciar(context.Background(), k.ID, "lenta", hash[:]); err != nil {
		t.Fatal(err)
	}
	rec := a.chamarBruto(http.MethodPost, "/cob", corpoCob, map[string]string{"Idempotency-Key": "lenta"})
	if rec.Code != http.StatusConflict || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "RequisicaoEmAndamento") {
		t.Fatalf("status %d, corpo %s", rec.Code, rec.Body)
	}
	if n := a.totalCobs(); n != 0 {
		t.Fatalf("criou cobrança com a chave reservada: %d", n)
	}
}

func TestIdempotencyKeyNoPut(t *testing.T) {
	a := novaAPI(t)
	h := map[string]string{"Idempotency-Key": "put-1"}
	a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, h, nil)
	// Sem a chave, o segundo PUT seria 409 (txid repetido); com ela, repete o 201.
	if rec := a.chamarBruto(http.MethodPut, "/cob/"+txid, corpoCob, h); rec.Code != http.StatusCreated || rec.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatalf("status %d", rec.Code)
	}
	if st := a.chamar(http.MethodPut, "/cob/"+txid, corpoCob, nil, nil); st != http.StatusConflict {
		t.Fatalf("sem a chave: %d", st)
	}
}
