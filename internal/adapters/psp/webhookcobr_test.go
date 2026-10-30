package psp_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/domain"
)

// O exemplo cobRWebhookNotification1 da API Pix, com uma correção: a spec traz
// dataLiquidacao "2024-20-08" (mês 20), que não é data. Aqui vai "2024-08-20".
const exemploCobr = `{"cobsr":[{"idRec":"RR1234567820240115abcdefghijk","txid":"3136957d93134f2184b369e8f1c0729d","status":"ATIVA",
 "atualizacao":[{"status":"ATIVA","data":"2024-08-20T12:34:21.300Z"}],
 "tentativas":[{"dataLiquidacao":"2024-08-20","tipo":"AGND","status":"SOLICITADA","endToEndId":"E12345678202406201221abcdef12345"}],
 "valor":{"original":"100.00"},"valorLiquidoEstimado":"91.18"}]}`

func TestWebhookCobrBCB(t *testing.T) {
	ats, err := psp.WebhookCobrBCB(domain.Conta{}, []byte(exemploCobr))
	if err != nil || len(ats) != 1 {
		t.Fatalf("exemplo da spec: %+v, %v", ats, err)
	}
	a := ats[0]
	quer := domain.Tentativa{DataLiquidacao: domain.NovaData(2024, 8, 20), Tipo: domain.TipoAgendamento, Status: domain.TentativaSolicitada, EndToEndID: "E12345678202406201221abcdef12345"}
	if a.Txid != "3136957d93134f2184b369e8f1c0729d" || a.IDRec != "RR1234567820240115abcdefghijk" || a.Status != domain.CobrAtiva || len(a.Tentativas) != 1 || a.Tentativas[0] != quer {
		t.Fatalf("atualização = %+v", a)
	}

	rejeitada := `{"cobsr":[{"idRec":"RR1234567820240115abcdefghijk","txid":"3136957d93134f2184b369e8f1c0729d","status":"ATIVA",
	 "tentativas":[{"dataLiquidacao":"2024-08-20","tipo":"AGND","status":"REJEITADA","endToEndId":"E1","rejeicao":{"codigo":"AM04","descricao":"Saldo insuficiente"}}]},
	 {"idRec":"RR1234567820240115abcdefghijk","txid":"3136957d93134f2184b369e8f1c0729e","status":"CANCELADA",
	 "encerramento":{"cancelamento":{"solicitante":"USUARIO_RECEBEDOR","codigo":"SLCR","descricao":"Cancelada pelo recebedor"}}}]}`
	ats, err = psp.WebhookCobrBCB(domain.Conta{}, []byte(rejeitada))
	if err != nil || ats[0].Tentativas[0].MotivoRejeicao != "AM04: Saldo insuficiente" || ats[1].MotivoEncerramento != "SLCR: Cancelada pelo recebedor" {
		t.Fatalf("motivos: %+v, %v", ats, err)
	}

	original := strings.Replace(exemploCobr, "2024-08-20\",\"tipo", "2024-20-08\",\"tipo", 1)
	for nome, ruim := range map[string]string{
		"não é json":              `não é json`,
		"lista vazia":             `{"cobsr":[]}`,
		"txid curto":              strings.Replace(exemploCobr, "3136957d93134f2184b369e8f1c0729d", "curto", 1),
		"idRec fora da regra":     strings.Replace(exemploCobr, "RR1234567820240115abcdefghijk", "RX1", 1),
		"status inventado":        strings.Replace(exemploCobr, `0729d","status":"ATIVA"`, `0729d","status":"PAGA"`, 1),
		"tipo inventado":          strings.Replace(exemploCobr, `"AGND"`, `"XPTO"`, 1),
		"data do exemplo da spec": original,
	} {
		if _, err := psp.WebhookCobrBCB(domain.Conta{}, []byte(ruim)); !errors.Is(err, domain.ErrCobrancaRecorrenteInvalida) {
			t.Errorf("%s: err = %v", nome, err)
		}
	}
}
