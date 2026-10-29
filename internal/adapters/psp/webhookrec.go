package psp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
)

// webhookRec é o corpo do callback de recorrência da spec do BCB: POST {webhookUrl}/rec.
type webhookRec struct {
	Recs []struct {
		IDRec       string `json:"idRec"`
		Status      string `json:"status"`
		Atualizacao []struct {
			Status string `json:"status"`
			Data   string `json:"data"`
		} `json:"atualizacao"`
		Encerramento *struct {
			Rejeicao *struct {
				Codigo    string `json:"codigo"`
				Descricao string `json:"descricao"`
			} `json:"rejeicao"`
		} `json:"encerramento"`
	} `json:"recs"`
}

var statusRec = map[string]bool{"CRIADA": true, "APROVADA": true, "REJEITADA": true, "EXPIRADA": true, "CANCELADA": true}

// WebhookRecBCB interpreta o callback de recorrência no formato da spec do
// BCB. Um item inválido invalida o lote, como no webhook de Pix.
func WebhookRecBCB(_ domain.Conta, corpo []byte) ([]domain.AtualizacaoRecorrencia, error) {
	var w webhookRec
	if err := json.Unmarshal(corpo, &w); err != nil {
		return nil, fmt.Errorf("%w: corpo não é o JSON da spec: %w", domain.ErrRecorrenciaInvalida, err)
	}
	if len(w.Recs) == 0 {
		return nil, fmt.Errorf("%w: lista recs vazia", domain.ErrRecorrenciaInvalida)
	}
	out := make([]domain.AtualizacaoRecorrencia, 0, len(w.Recs))
	for i, r := range w.Recs {
		if err := domain.ValidarIDRecorrencia(r.IDRec); err != nil {
			return nil, fmt.Errorf("recs[%d]: %w", i, err)
		}
		if !statusRec[r.Status] {
			return nil, fmt.Errorf("recs[%d]: %w: status %q", i, domain.ErrRecorrenciaInvalida, r.Status)
		}
		a := domain.AtualizacaoRecorrencia{IDRec: r.IDRec, Status: domain.StatusRecorrencia(r.Status)}
		// O instante da mudança é o da última atualização com o status avisado.
		for _, at := range r.Atualizacao {
			if at.Status != r.Status {
				continue
			}
			t, err := time.Parse(time.RFC3339, at.Data)
			if err != nil {
				return nil, fmt.Errorf("recs[%d]: %w: atualizacao.data %q não é RFC 3339", i, domain.ErrRecorrenciaInvalida, at.Data)
			}
			a.Em = t.UTC()
		}
		if e := r.Encerramento; e != nil && e.Rejeicao != nil {
			a.MotivoRejeicao = e.Rejeicao.Codigo
			if e.Rejeicao.Descricao != "" {
				a.MotivoRejeicao += ": " + e.Rejeicao.Descricao
			}
		}
		out = append(out, a)
	}
	return out, nil
}
