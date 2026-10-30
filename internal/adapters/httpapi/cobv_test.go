package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
)

const txidV = "boleto000000000000000000001"

// O relógio dos testes HTTP está em 29/10/2026.
const corpoCobv = `{
  "calendario": {"dataDeVencimento": "2026-11-30", "validadeAposVencimento": 10},
  "devedor": {"cnpj": "12ABC34501DE35", "nome": "Empresa Alfanumérica", "uf": "PE", "cep": "50000000"},
  "valor": {
    "original": "150.00",
    "multa": {"modalidade": 2, "valorPerc": "2.00"},
    "juros": {"modalidade": 3, "valorPerc": "1.00"},
    "desconto": {"modalidade": 1, "descontoDataFixa": [{"data": "2026-11-20", "valorPerc": "10.00"}]}
  },
  "chave": "loja@example.com",
  "solicitacaoPagador": "Mensalidade de novembro"
}`

func TestCobvFluxoCompletoPelaAPI(t *testing.T) {
	a := novaAPI(t)
	var v gen.CobVGerada
	if st := a.chamar(http.MethodPut, "/cobv/"+txidV, corpoCobv, nil, &v); st != http.StatusCreated {
		t.Fatalf("PUT status = %d", st)
	}
	if v.Status != gen.CobVStatusATIVA || v.Valor.Original != "150.00" || v.Calendario.DataDeVencimento.String() != "2026-11-30" ||
		v.Valor.Multa == nil || v.Valor.Multa.ValorPerc != "2.00" || v.Valor.Desconto == nil || v.Valor.Desconto.DescontoDataFixa == nil ||
		v.Devedor.Cnpj == nil || *v.Devedor.Cnpj != "12ABC34501DE35" || v.Loc == nil || v.PixCopiaECola == nil {
		t.Fatalf("criada: %+v", v)
	}
	var c gen.CobVCompleta
	a.clock.Avancar(sandbox.AtrasoPadrao)
	if st := a.chamar(http.MethodGet, "/cobv/"+txidV, "", nil, &c); st != http.StatusOK || c.Status != gen.CobVStatusCONCLUIDA {
		t.Fatalf("depois de pagar: %d %s", st, c.Status)
	}
	var lista gen.CobVsConsultadas
	if st := a.chamar(http.MethodGet, "/cobv?inicio=2026-10-29T00:00:00Z&fim=2026-10-30T00:00:00Z", "", nil, &lista); st != http.StatusOK || len(lista.Cobs) != 1 {
		t.Fatalf("listar: %d %d", st, len(lista.Cobs))
	}
}

func TestCobvPatchERemocao(t *testing.T) {
	a := novaAPI(t)
	a.chamar(http.MethodPut, "/cobv/"+txidV, corpoCobv, nil, nil)
	var v gen.CobVGerada
	patch := `{"calendario": {"dataDeVencimento": "2026-12-10"}, "valor": {"original": "180.00", "multa": {"modalidade": 1, "valorPerc": "5.00"}}}`
	if st := a.chamar(http.MethodPatch, "/cobv/"+txidV, patch, nil, &v); st != http.StatusOK || v.Valor.Original != "180.00" || v.Calendario.DataDeVencimento.String() != "2026-12-10" || v.Valor.Multa.Modalidade != 1 || v.Revisao != 1 {
		t.Fatalf("revisão: %d %+v", st, v)
	}
	if st := a.chamar(http.MethodPatch, "/cobv/"+txidV, `{"status": "REMOVIDA_PELO_USUARIO_RECEBEDOR"}`, nil, &v); st != http.StatusOK || v.Status != gen.CobVStatusREMOVIDAPELOUSUARIORECEBEDOR {
		t.Fatalf("remoção: %d %s", st, v.Status)
	}
}

func TestCobvViolacoesEEscopo(t *testing.T) {
	a := novaAPI(t)
	var p gen.Problema
	invalido := strings.Replace(corpoCobv, `"modalidade": 2, "valorPerc": "2.00"`, `"modalidade": 7, "valorPerc": "2.00"`, 1)
	invalido = strings.Replace(invalido, `"2026-11-30"`, `"2026-10-01"`, 1)
	if st := a.chamar(http.MethodPut, "/cobv/"+txidV, invalido, nil, &p); st != http.StatusBadRequest || p.Violacoes == nil || len(*p.Violacoes) < 2 {
		t.Fatalf("status %d, %+v", st, p)
	}
	soCob := a.novoToken("", "cob:write")
	if st := a.chamar(http.MethodPut, "/cobv/"+txidV, corpoCobv, map[string]string{"Authorization": "Bearer " + soCob}, &p); st != http.StatusForbidden {
		t.Fatalf("sem cobv:write: %d", st)
	}
}
