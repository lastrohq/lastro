package psp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
)

// webhookPix é o corpo do callback da spec do BCB: POST {webhookUrl}/pix.
type webhookPix struct {
	Pix []struct {
		EndToEndID  string          `json:"endToEndId"`
		Txid        string          `json:"txid"`
		Valor       string          `json:"valor"`
		Chave       string          `json:"chave"`
		Horario     string          `json:"horario"`
		InfoPagador string          `json:"infoPagador"`
		Devolucoes  []devolucaoSpec `json:"devolucoes"`
	} `json:"pix"`
}

// devolucaoSpec é uma devolução como a spec a descreve, dentro do Pix.
type devolucaoSpec struct {
	ID       string `json:"id"`
	RtrID    string `json:"rtrId"`
	Valor    string `json:"valor"`
	Natureza string `json:"natureza"`
	Horario  struct {
		Solicitacao string `json:"solicitacao"`
		Liquidacao  string `json:"liquidacao"`
	} `json:"horario"`
	Status string `json:"status"`
	Motivo string `json:"motivo"`
}

// devolucaoBCB converte a devolução do formato da spec. Status fora da RN-20 é
// erro; horário de liquidação ausente fica zero.
func devolucaoBCB(e2e string, d devolucaoSpec) (domain.Devolucao, error) {
	if err := domain.ValidarIDDevolucao(d.ID); err != nil {
		return domain.Devolucao{}, fmt.Errorf("%w: devolução: %w", domain.ErrPixInvalido, err)
	}
	valor, err := domain.ParseMoney(d.Valor)
	if err != nil || !valor.Valid() {
		return domain.Devolucao{}, fmt.Errorf("%w: devolução %s: valor %q", domain.ErrPixInvalido, d.ID, d.Valor)
	}
	status := domain.StatusDevolucao(d.Status)
	switch status {
	case domain.DevolucaoEmProcessamento, domain.DevolucaoDevolvido, domain.DevolucaoNaoRealizado:
	default:
		return domain.Devolucao{}, fmt.Errorf("%w: devolução %s: status %q", domain.ErrPixInvalido, d.ID, d.Status)
	}
	out := domain.Devolucao{ID: d.ID, EndToEndID: e2e, RtrID: d.RtrID, Valor: valor, Natureza: domain.NaturezaDevolucao(d.Natureza), Status: status, Motivo: d.Motivo}
	if d.Horario.Liquidacao != "" {
		t, err := time.Parse(time.RFC3339, d.Horario.Liquidacao)
		if err != nil {
			return domain.Devolucao{}, fmt.Errorf("%w: devolução %s: horario.liquidacao %q não é RFC 3339", domain.ErrPixInvalido, d.ID, d.Horario.Liquidacao)
		}
		out.LiquidadaEm = t.UTC()
	}
	return out, nil
}

// WebhookPixBCB interpreta o callback no formato da spec do BCB. Um item
// inválido invalida o lote: melhor o banco reenviar que perder um Pix em silêncio.
func WebhookPixBCB(conta domain.Conta, corpo []byte) ([]domain.PixRecebido, error) {
	var w webhookPix
	if err := json.Unmarshal(corpo, &w); err != nil {
		return nil, fmt.Errorf("%w: corpo não é o JSON da spec: %w", domain.ErrPixInvalido, err)
	}
	if len(w.Pix) == 0 {
		return nil, fmt.Errorf("%w: lista pix vazia", domain.ErrPixInvalido)
	}
	out := make([]domain.PixRecebido, 0, len(w.Pix))
	for i, p := range w.Pix {
		if err := domain.ValidarEndToEndID(p.EndToEndID); err != nil {
			return nil, fmt.Errorf("pix[%d]: %w", i, err)
		}
		valor, err := domain.ParseMoney(p.Valor)
		if err != nil || !valor.Valid() {
			return nil, fmt.Errorf("pix[%d]: %w: valor %q", i, domain.ErrPixInvalido, p.Valor)
		}
		horario, err := time.Parse(time.RFC3339, p.Horario)
		if err != nil {
			return nil, fmt.Errorf("pix[%d]: %w: horario %q não é RFC 3339", i, domain.ErrPixInvalido, p.Horario)
		}
		// txid é opcional (Pix por chave, sem cobrança) e pode ter de 1 a 35 caracteres no Pix.
		px := domain.PixRecebido{
			EndToEndID: p.EndToEndID, ContaID: conta.ID, Txid: domain.Txid(p.Txid), Valor: valor,
			Chave: p.Chave, Horario: horario.UTC(), InfoPagador: p.InfoPagador,
		}
		for _, d := range p.Devolucoes {
			dev, err := devolucaoBCB(p.EndToEndID, d)
			if err != nil {
				return nil, fmt.Errorf("pix[%d]: %w", i, err)
			}
			dev.ContaID = conta.ID
			px.Devolucoes = append(px.Devolucoes, dev)
		}
		out = append(out, px)
	}
	return out, nil
}
