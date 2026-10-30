package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/domain"
)

const txidCobr = "cobrapi000000000000000000001"

// O relógio da API está em 29/10/2026: a janela de antecedência vai de 31/10 a 08/11.
const corpoCobr = `{"idRec":"%s","calendario":{"dataDeVencimento":"2026-11-05"},"valor":{"original":"%s"},"infoAdicional":"Mensalidade"}`

// recAprovada cria pela API uma recorrência mensal a partir de 01/11, com o
// valor fixo dado, e a aprova direto no store, como faria o aviso do banco.
func (a *api) recAprovada(valor string) string {
	a.t.Helper()
	corpo := strings.NewReplacer(`"2026-11-10"`, `"2026-11-01"`, `"99.90"`, `"`+valor+`"`).Replace(pedidoRec)
	var r gen.RecGerada
	if st := a.chamar(http.MethodPost, "/rec", corpo, comChaveDaLoja(a, "rec-"+valor), &r); st != http.StatusCreated {
		a.t.Fatalf("criar recorrência: %d", st)
	}
	rec, _ := a.store.Recorrencias().Buscar(a.t.Context(), r.IdRec)
	rec.Status = domain.RecorrenciaAprovada
	if err := a.store.Recorrencias().Atualizar(a.t.Context(), &rec); err != nil {
		a.t.Fatal(err)
	}
	return r.IdRec
}

func comCobr(a *api, conta, chave string) map[string]string {
	h := map[string]string{"Authorization": "Bearer " + a.novoToken(conta, domain.EscopoCobrRead, domain.EscopoCobrWrite)}
	if chave != "" {
		h["Idempotency-Key"] = chave
	}
	return h
}

func corpoDaCobr(idRec, valor string) string {
	return strings.NewReplacer("%s", idRec).Replace(strings.Replace(corpoCobr, `"original":"%s"`, `"original":"`+valor+`"`, 1))
}

func TestCriarConsultarECancelarCobr(t *testing.T) {
	a := novaAPI(t)
	idRec := a.recAprovada("99.90")
	h := comCobr(a, "loja", "")
	var c gen.CobRGerada
	if st := a.chamar(http.MethodPut, "/cobr/"+txidCobr, corpoDaCobr(idRec, "99.90"), h, &c); st != http.StatusCreated {
		t.Fatalf("criar: %d", st)
	}
	if c.Txid != txidCobr || c.IdRec != idRec || c.Status != "CRIADA" || !c.AjusteDiaUtil || deref(c.InfoAdicional) != "Mensalidade" ||
		c.Valor.Original != "99.90" || c.Calendario.DataDeVencimento.String() != "2026-11-05" || len(c.Tentativas) != 0 {
		t.Fatalf("criada = %+v", c)
	}
	var lida gen.CobRGerada
	if st := a.chamar(http.MethodGet, "/cobr/"+txidCobr, "", h, &lida); st != http.StatusOK || lida.Txid != txidCobr {
		t.Fatalf("consultar: %d %+v", st, lida)
	}
	var cancelada gen.CobRGerada
	if st := a.chamar(http.MethodPatch, "/cobr/"+txidCobr, `{"status":"CANCELADA"}`, h, &cancelada); st != http.StatusOK || cancelada.Status != "CANCELADA" || cancelada.MotivoEncerramento == nil {
		t.Fatalf("cancelar: %d %+v", st, cancelada)
	}
	var p gen.Problema
	if st := a.chamar(http.MethodPatch, "/cobr/"+txidCobr, `{"status":"CANCELADA"}`, h, &p); st != http.StatusBadRequest {
		t.Fatalf("cancelar de novo: %d", st)
	}
	if st := a.chamar(http.MethodPatch, "/cobr/"+txidCobr, `{"status":"CONCLUIDA"}`, h, &p); st != http.StatusBadRequest {
		t.Fatalf("revisão que não é cancelamento: %d", st)
	}
}

func TestCriarCobrSemTxidEIdempotente(t *testing.T) {
	a := novaAPI(t)
	idRec := a.recAprovada("99.90")
	h := comCobr(a, "loja", "cobr-1")
	var c, repetida gen.CobRGerada
	if st := a.chamar(http.MethodPost, "/cobr", corpoDaCobr(idRec, "99.90"), h, &c); st != http.StatusCreated || len(c.Txid) != 32 {
		t.Fatalf("criar: %d %+v", st, c)
	}
	if st := a.chamar(http.MethodPost, "/cobr", corpoDaCobr(idRec, "99.90"), h, &repetida); st != http.StatusCreated || repetida.Txid != c.Txid {
		t.Fatalf("mesma Idempotency-Key: %d %s × %s", st, repetida.Txid, c.Txid)
	}
	var p gen.Problema
	if st := a.chamar(http.MethodPost, "/cobr", corpoDaCobr(idRec, "99.90"), comCobr(a, "loja", ""), &p); st != http.StatusBadRequest {
		t.Fatalf("sem Idempotency-Key: %d", st)
	}
}

func TestCobrRecusada(t *testing.T) {
	a := novaAPI(t)
	idRec := a.recAprovada("99.90")
	if err := a.store.Contas().Criar(t.Context(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	var naoAprovada gen.RecGerada
	_ = a.chamar(http.MethodPost, "/rec", strings.Replace(pedidoRec, `"2026-11-10"`, `"2026-11-01"`, 1), comChaveDaLoja(a, "rec-x"), &naoAprovada)
	casos := map[string]struct {
		txid, corpo string
		header      map[string]string
		quer        int
	}{
		"recorrência não aprovada":   {"cobrapi000000000000000000002", corpoDaCobr(naoAprovada.IdRec, "99.90"), comCobr(a, "loja", ""), http.StatusBadRequest},
		"fora da antecedência":       {"cobrapi000000000000000000003", strings.Replace(corpoDaCobr(idRec, "99.90"), "2026-11-05", "2026-11-20", 1), comCobr(a, "loja", ""), http.StatusBadRequest},
		"valor diferente do fixo":    {"cobrapi000000000000000000004", corpoDaCobr(idRec, "10.00"), comCobr(a, "loja", ""), http.StatusBadRequest},
		"recorrência de outra conta": {"cobrapi000000000000000000005", corpoDaCobr(idRec, "99.90"), comCobr(a, "outra", ""), http.StatusNotFound},
		"sem cobr:write": {"cobrapi000000000000000000006", corpoDaCobr(idRec, "99.90"),
			map[string]string{"Authorization": "Bearer " + a.novoToken("loja", domain.EscopoCobrRead, domain.EscopoRecWrite)}, http.StatusForbidden},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			var p gen.Problema
			if st := a.chamar(http.MethodPut, "/cobr/"+c.txid, c.corpo, c.header, &p); st != c.quer {
				t.Fatalf("status %d, quer %d: %+v", st, c.quer, p)
			}
		})
	}
	// O ciclo de novembro já tem cobrança: outra no mesmo ciclo é recusada.
	_ = a.chamar(http.MethodPut, "/cobr/"+txidCobr, corpoDaCobr(idRec, "99.90"), comCobr(a, "loja", ""), nil)
	var p gen.Problema
	if st := a.chamar(http.MethodPut, "/cobr/cobrapi000000000000000000007", strings.Replace(corpoDaCobr(idRec, "99.90"), "2026-11-05", "2026-11-06", 1), comCobr(a, "loja", ""), &p); st != http.StatusBadRequest {
		t.Fatalf("segundo do ciclo: %d", st)
	}
	if st := a.chamar(http.MethodPut, "/cobr/"+txidCobr, corpoDaCobr(idRec, "99.90"), comCobr(a, "loja", ""), &p); st != http.StatusConflict {
		t.Fatalf("txid repetido: %d", st)
	}
	if st := a.chamar(http.MethodGet, "/cobr/"+txidCobr, "", comCobr(a, "outra", ""), &p); st != http.StatusNotFound {
		t.Fatalf("outra conta enxergou: %d", st)
	}
}

func TestRetentativaCobrPelaAPI(t *testing.T) {
	a := novaAPI(t)
	// R$ 0,55: no sandbox, a tentativa do vencimento falha por falta de saldo.
	idRec := a.recAprovada("0.55")
	h := comCobr(a, "loja", "")
	if st := a.chamar(http.MethodPut, "/cobr/"+txidCobr, corpoDaCobr(idRec, "0.55"), h, nil); st != http.StatusCreated {
		t.Fatalf("criar: %d", st)
	}
	// O aviso de que a tentativa falhou chega com o webhook de cobrança
	// recorrente; aqui ele é gravado direto no store.
	c, _ := a.store.CobrancasRecorrentes().BuscarPorTxid(t.Context(), txidCobr)
	c.Status = domain.CobrAtiva
	c.Tentativas = []domain.Tentativa{{DataLiquidacao: c.Vencimento, Tipo: domain.TipoAgendamento, Status: domain.TentativaRejeitada}}
	if err := a.store.CobrancasRecorrentes().Atualizar(t.Context(), &c); err != nil {
		t.Fatal(err)
	}
	a.clock.Avancar(time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC).Sub(a.clock.Now()))

	var got gen.CobRGerada
	if st := a.chamar(http.MethodPost, "/cobr/"+txidCobr+"/retentativa/2026-11-07", "", h, &got); st != http.StatusCreated {
		t.Fatalf("retentativa: %d", st)
	}
	if len(got.Tentativas) != 2 || got.Tentativas[1].Tipo != "NTAG" || got.Tentativas[1].Status != "AGENDADA" || got.Tentativas[1].DataLiquidacao.String() != "2026-11-07" {
		t.Fatalf("tentativas = %+v", got.Tentativas)
	}
	var p gen.Problema
	if st := a.chamar(http.MethodPost, "/cobr/"+txidCobr+"/retentativa/2026-11-07", "", h, &p); st != http.StatusBadRequest {
		t.Fatalf("mesma data de novo: %d", st)
	}
	if st := a.chamar(http.MethodPost, "/cobr/"+txidCobr+"/retentativa/2026-11-20", "", h, &p); st != http.StatusBadRequest {
		t.Fatalf("fora dos 7 dias: %d", st)
	}
}
