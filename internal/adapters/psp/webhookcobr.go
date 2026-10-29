package psp

import (
	"encoding/json"
	"fmt"

	"github.com/lastrohq/lastro/internal/domain"
)

// encerramento é o motivo da rejeição ou do cancelamento, no formato da spec.
type encerramento struct {
	Codigo    string `json:"codigo"`
	Descricao string `json:"descricao"`
}

func (e *encerramento) texto() string {
	if e == nil || e.Codigo == "" {
		return ""
	}
	if e.Descricao == "" {
		return e.Codigo
	}
	return e.Codigo + ": " + e.Descricao
}

// CobrBCB é a cobrança recorrente como a spec a devolve na consulta (GET
// /cobr/{txid}) e no callback (CobRNotification), só com o que o Lastro usa.
type CobrBCB struct {
	IDRec        string `json:"idRec"`
	Txid         string `json:"txid"`
	Status       string `json:"status"`
	Encerramento *struct {
		Rejeicao     *encerramento `json:"rejeicao"`
		Cancelamento *encerramento `json:"cancelamento"`
	} `json:"encerramento"`
	Tentativas []struct {
		DataLiquidacao string        `json:"dataLiquidacao"`
		Tipo           string        `json:"tipo"`
		Status         string        `json:"status"`
		EndToEndID     string        `json:"endToEndId"`
		Rejeicao       *encerramento `json:"rejeicao"`
	} `json:"tentativas"`
}

var (
	statusCobr      = map[string]bool{"CRIADA": true, "ATIVA": true, "CONCLUIDA": true, "EXPIRADA": true, "REJEITADA": true, "CANCELADA": true}
	statusTentativa = map[string]bool{"SOLICITADA": true, "AGENDADA": true, "PAGA": true, "CANCELADA": true, "REJEITADA": true, "EXPIRADA": true}
	tipoTentativa   = map[string]bool{"AGND": true, "NTAG": true, "RIFL": true}
)

// Atualizacao confere a cobrança e a converte para o domínio. Qualquer campo
// fora da spec invalida a cobrança inteira.
func (c CobrBCB) Atualizacao() (domain.AtualizacaoCobrancaRecorrente, error) {
	falha := func(format string, args ...any) (domain.AtualizacaoCobrancaRecorrente, error) {
		return domain.AtualizacaoCobrancaRecorrente{}, fmt.Errorf("%w: "+format, append([]any{domain.ErrCobrancaRecorrenteInvalida}, args...)...)
	}
	txid, err := domain.ParseTxid(c.Txid)
	if err != nil {
		return falha("txid: %v", err)
	}
	if err := domain.ValidarIDRecorrencia(c.IDRec); err != nil {
		return falha("%v", err)
	}
	if !statusCobr[c.Status] {
		return falha("status %q", c.Status)
	}
	a := domain.AtualizacaoCobrancaRecorrente{Txid: txid, IDRec: c.IDRec, Status: domain.StatusCobrancaRecorrente(c.Status)}
	if e := c.Encerramento; e != nil {
		a.MotivoEncerramento = e.Rejeicao.texto()
		if a.MotivoEncerramento == "" {
			a.MotivoEncerramento = e.Cancelamento.texto()
		}
	}
	for i, t := range c.Tentativas {
		d, err := domain.ParseData(t.DataLiquidacao)
		if err != nil {
			return falha("tentativas[%d].dataLiquidacao: %v", i, err)
		}
		if !tipoTentativa[t.Tipo] || !statusTentativa[t.Status] {
			return falha("tentativas[%d]: tipo %q, status %q", i, t.Tipo, t.Status)
		}
		a.Tentativas = append(a.Tentativas, domain.Tentativa{
			DataLiquidacao: d, Tipo: domain.TipoTentativa(t.Tipo), Status: domain.StatusTentativa(t.Status),
			EndToEndID: t.EndToEndID, MotivoRejeicao: t.Rejeicao.texto(),
		})
	}
	return a, nil
}

// WebhookCobrBCB interpreta o callback de cobrança recorrente da spec do BCB
// (POST {webhookUrl}/cobr, com {"cobsr": [...]}). Um item inválido invalida o
// lote, como no webhook de Pix: melhor o banco reenviar que perder um aviso.
func WebhookCobrBCB(_ domain.Conta, corpo []byte) ([]domain.AtualizacaoCobrancaRecorrente, error) {
	var w struct {
		Cobsr []CobrBCB `json:"cobsr"`
	}
	if err := json.Unmarshal(corpo, &w); err != nil {
		return nil, fmt.Errorf("%w: corpo não é o JSON da spec: %w", domain.ErrCobrancaRecorrenteInvalida, err)
	}
	if len(w.Cobsr) == 0 {
		return nil, fmt.Errorf("%w: lista cobsr vazia", domain.ErrCobrancaRecorrenteInvalida)
	}
	out := make([]domain.AtualizacaoCobrancaRecorrente, len(w.Cobsr))
	for i, c := range w.Cobsr {
		a, err := c.Atualizacao()
		if err != nil {
			return nil, fmt.Errorf("cobsr[%d]: %w", i, err)
		}
		out[i] = a
	}
	return out, nil
}
