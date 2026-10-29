package httpapi

import (
	"context"
	"encoding/json"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
)

// ReceberWebhookPix recebe o aviso de Pix enviado pelo banco.
func (s *Server) ReceberWebhookPix(ctx context.Context, r gen.ReceberWebhookPixRequestObject) (gen.ReceberWebhookPixResponseObject, error) {
	if err := s.receberWebhook(ctx, r.ContaId, r.Token, r.Body); err != nil {
		return nil, err
	}
	return gen.ReceberWebhookPix200Response{}, nil
}

// ReceberWebhookPixComSufixo é o mesmo aviso no caminho com /pix no fim.
func (s *Server) ReceberWebhookPixComSufixo(ctx context.Context, r gen.ReceberWebhookPixComSufixoRequestObject) (gen.ReceberWebhookPixComSufixoResponseObject, error) {
	if err := s.receberWebhook(ctx, r.ContaId, r.Token, r.Body); err != nil {
		return nil, err
	}
	return gen.ReceberWebhookPixComSufixo200Response{}, nil
}

func (s *Server) receberWebhook(ctx context.Context, conta, token string, corpo *gen.ReceberWebhookPixJSONBody) error {
	// O contrato aceita o corpo como objeto livre; o adapter do banco recebe os bytes.
	b, err := json.Marshal(corpo)
	if err != nil {
		return err
	}
	_, err = s.webhooks.Receber(ctx, conta, token, b)
	return err
}
