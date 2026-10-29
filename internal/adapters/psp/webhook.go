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
		EndToEndID  string `json:"endToEndId"`
		Txid        string `json:"txid"`
		Valor       string `json:"valor"`
		Chave       string `json:"chave"`
		Horario     string `json:"horario"`
		InfoPagador string `json:"infoPagador"`
	} `json:"pix"`
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
		out = append(out, domain.PixRecebido{
			EndToEndID: p.EndToEndID, ContaID: conta.ID, Txid: domain.Txid(p.Txid), Valor: valor,
			Chave: p.Chave, Horario: horario.UTC(), InfoPagador: p.InfoPagador,
		})
	}
	return out, nil
}
