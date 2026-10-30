package psp_test

import (
	"errors"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/domain"
)

// O exemplo recWebhookNotification1 da API Pix 2.10.0.
const exemploRec = `{"recs":[{"idRec":"RR1026652320240821lab77511abf","status":"APROVADA",
 "atualizacao":[{"status":"CRIADA","data":"2024-08-20T10:12:07.567Z"},{"status":"APROVADA","data":"2024-08-22T12:43:53.337Z"}],
 "ativacao":{"tipoJornada":"JORNADA_3","dadosJornada":{"txid":"r9eFIFmwcZ55Nm4RsKZAAtIvvCrlcNN6"}}}]}`

func TestWebhookRecBCB(t *testing.T) {
	ats, err := psp.WebhookRecBCB(domain.Conta{}, []byte(exemploRec))
	if err != nil || len(ats) != 1 {
		t.Fatalf("exemplo da spec: %+v, %v", ats, err)
	}
	quer := domain.AtualizacaoRecorrencia{IDRec: "RR1026652320240821lab77511abf", Status: domain.RecorrenciaAprovada, Em: time.Date(2024, 8, 22, 12, 43, 53, 337000000, time.UTC)}
	if ats[0] != quer {
		t.Fatalf("atualização = %+v\nquer          %+v", ats[0], quer)
	}

	rejeitada := `{"recs":[{"idRec":"RN1026652320240821lab77511abf","status":"REJEITADA","atualizacao":[],"encerramento":{"rejeicao":{"codigo":"AP13","descricao":"Pagador não reconhece o recebedor"}}}]}`
	ats, err = psp.WebhookRecBCB(domain.Conta{}, []byte(rejeitada))
	if err != nil || ats[0].MotivoRejeicao != "AP13: Pagador não reconhece o recebedor" || !ats[0].Em.IsZero() {
		t.Fatalf("rejeitada: %+v, %v", ats, err)
	}

	for _, ruim := range []string{`não é json`, `{"recs":[]}`, `{"recs":[{"idRec":"curto","status":"APROVADA"}]}`,
		`{"recs":[{"idRec":"RR1026652320240821lab77511abf","status":"PAGA"}]}`,
		`{"recs":[{"idRec":"RR1026652320240821lab77511abf","status":"APROVADA","atualizacao":[{"status":"APROVADA","data":"ontem"}]}]}`} {
		if _, err := psp.WebhookRecBCB(domain.Conta{}, []byte(ruim)); !errors.Is(err, domain.ErrRecorrenciaInvalida) {
			t.Errorf("%s: err = %v", ruim, err)
		}
	}
}
