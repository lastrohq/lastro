package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/domain"
)

const pedidoRec = `{"vinculo":{"contrato":"63100862","objeto":"Academia","devedor":{"cpf":"12345678909","nome":"Fulano de Tal"}},
	"calendario":{"dataInicial":"2026-11-10","periodicidade":"MENSAL"},"valor":{"valorRec":"99.90"},"politicaRetentativa":"PERMITE_3R_7D"}`

// comChaveDaLoja usa uma API key restrita à conta "loja", que identifica o recebedor.
func comChaveDaLoja(a *api, chave string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + a.novoToken("loja", domain.EscopoRecWrite, domain.EscopoRecRead), "Idempotency-Key": chave}
}

func TestCriarEConsultarRec(t *testing.T) {
	a := novaAPI(t)
	h := comChaveDaLoja(a, "rec-1")
	var r gen.RecGerada
	if st := a.chamar(http.MethodPost, "/rec", pedidoRec, h, &r); st != http.StatusCreated {
		t.Fatalf("criar: %d", st)
	}
	if err := domain.ValidarIDRecorrencia(r.IdRec); err != nil || !strings.HasPrefix(r.IdRec, "RR") || r.Status != "CRIADA" {
		t.Fatalf("recorrência = %+v (%v)", r, err)
	}
	if r.Valor == nil || r.Valor.ValorRec == nil || *r.Valor.ValorRec != "99.90" || r.Vinculo.Devedor.Cpf == nil {
		t.Fatalf("campos não voltaram: %+v", r)
	}

	var lida gen.RecGerada
	if st := a.chamar(http.MethodGet, "/rec/"+r.IdRec, "", h, &lida); st != http.StatusOK || lida.IdRec != r.IdRec {
		t.Fatalf("consultar: %d %+v", st, lida)
	}
	// Mesma Idempotency-Key: devolve a mesma recorrência, sem criar outra.
	var repetida gen.RecGerada
	if st := a.chamar(http.MethodPost, "/rec", pedidoRec, h, &repetida); st != http.StatusCreated || repetida.IdRec != r.IdRec {
		t.Fatalf("repetida: %d %s × %s", st, repetida.IdRec, r.IdRec)
	}
}

func TestRecExigeChaveRestrita(t *testing.T) {
	a := novaAPI(t)
	var p gen.Problema
	// a.token é irrestrito: não diz qual conta é o recebedor.
	if st := a.chamar(http.MethodPost, "/rec", pedidoRec, map[string]string{"Idempotency-Key": "rec-2"}, &p); st != http.StatusBadRequest {
		t.Fatalf("status %d", st)
	}
	if p.Violacoes == nil || len(*p.Violacoes) != 1 || deref((*p.Violacoes)[0].Propriedade) != "Authorization" {
		t.Fatalf("problema = %+v", p)
	}
}

func TestRecInvalidaERotasProtegidas(t *testing.T) {
	a := novaAPI(t)
	var p gen.Problema
	fixoEPiso := strings.Replace(pedidoRec, `"valorRec":"99.90"`, `"valorRec":"99.90","valorMinimoRecebedor":"50.00"`, 1)
	if st := a.chamar(http.MethodPost, "/rec", fixoEPiso, comChaveDaLoja(a, "rec-3"), &p); st != http.StatusBadRequest || !strings.Contains(deref(p.Detail), "excludentes") {
		t.Fatalf("fixo e piso: %d %+v", st, p)
	}
	if st := a.chamar(http.MethodPost, "/rec", pedidoRec, map[string]string{"Authorization": comChaveDaLoja(a, "")["Authorization"]}, &p); st != http.StatusBadRequest {
		t.Fatalf("sem Idempotency-Key: %d", st)
	}
	soLeitura := map[string]string{"Authorization": "Bearer " + a.novoToken("loja", domain.EscopoRecRead), "Idempotency-Key": "rec-4"}
	if st := a.chamar(http.MethodPost, "/rec", pedidoRec, soLeitura, &p); st != http.StatusForbidden {
		t.Fatalf("sem rec:write: %d", st)
	}
}

func TestRecDeOutraContaE404(t *testing.T) {
	a := novaAPI(t)
	var r gen.RecGerada
	_ = a.chamar(http.MethodPost, "/rec", pedidoRec, comChaveDaLoja(a, "rec-5"), &r)
	if err := a.store.Contas().Criar(t.Context(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	daOutra := map[string]string{"Authorization": "Bearer " + a.novoToken("outra", domain.EscopoRecRead)}
	var p gen.Problema
	if st := a.chamar(http.MethodGet, "/rec/"+r.IdRec, "", daOutra, &p); st != http.StatusNotFound {
		t.Fatalf("outra conta enxergou: %d", st)
	}
}

func TestCriarSolicRec(t *testing.T) {
	a := novaAPI(t)
	var r gen.RecGerada
	_ = a.chamar(http.MethodPost, "/rec", pedidoRec, comChaveDaLoja(a, "rec-6"), &r)
	expira := a.clock.Now().Add(7 * 24 * 3600 * 1e9).UTC().Format("2006-01-02T15:04:05Z")
	corpo := `{"idRec":"` + r.IdRec + `","calendario":{"dataExpiracaoSolicitacao":"` + expira + `"},"destinatario":{"conta":"123456","ispbParticipante":"18236120","agencia":"0001"}}`
	var s gen.SolicRecGerada
	if st := a.chamar(http.MethodPost, "/solicrec", corpo, comChaveDaLoja(a, "sol-1"), &s); st != http.StatusCreated {
		t.Fatalf("solicitar: %d", st)
	}
	if !strings.HasPrefix(s.IdSolicRec, "SC") || len(s.IdSolicRec) != 29 || s.Status != "ENVIADA" || s.IdRec != r.IdRec {
		t.Fatalf("solicitação = %+v", s)
	}
	var p gen.Problema
	tarde := strings.Replace(corpo, expira, a.clock.Now().Add(40*24*3600*1e9).UTC().Format("2006-01-02T15:04:05Z"), 1)
	if st := a.chamar(http.MethodPost, "/solicrec", tarde, comChaveDaLoja(a, "sol-2"), &p); st != http.StatusBadRequest {
		t.Fatalf("passa de 30 dias: %d", st)
	}
}
