package httpapi

import (
	"context"
	"encoding/json"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
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

// ReceberWebhookRec recebe o aviso de recorrência enviado pelo banco.
func (s *Server) ReceberWebhookRec(ctx context.Context, r gen.ReceberWebhookRecRequestObject) (gen.ReceberWebhookRecResponseObject, error) {
	if err := s.receberWebhookRec(ctx, r.ContaId, r.Token, r.Body); err != nil {
		return nil, err
	}
	return gen.ReceberWebhookRec200Response{}, nil
}

// ReceberWebhookRecComSufixo é o mesmo aviso no caminho com /rec no fim.
func (s *Server) ReceberWebhookRecComSufixo(ctx context.Context, r gen.ReceberWebhookRecComSufixoRequestObject) (gen.ReceberWebhookRecComSufixoResponseObject, error) {
	if err := s.receberWebhookRec(ctx, r.ContaId, r.Token, r.Body); err != nil {
		return nil, err
	}
	return gen.ReceberWebhookRecComSufixo200Response{}, nil
}

func (s *Server) receberWebhookRec(ctx context.Context, conta, token string, corpo *gen.ReceberWebhookRecJSONBody) error {
	b, err := json.Marshal(corpo)
	if err != nil {
		return err
	}
	_, err = s.webhooks.ReceberRec(ctx, conta, token, b)
	return err
}

// ReceberWebhookCobr recebe o aviso de cobrança recorrente enviado pelo banco.
func (s *Server) ReceberWebhookCobr(ctx context.Context, r gen.ReceberWebhookCobrRequestObject) (gen.ReceberWebhookCobrResponseObject, error) {
	if err := s.receberWebhookCobr(ctx, r.ContaId, r.Token, r.Body); err != nil {
		return nil, err
	}
	return gen.ReceberWebhookCobr200Response{}, nil
}

// ReceberWebhookCobrComSufixo é o mesmo aviso no caminho com /cobr no fim.
func (s *Server) ReceberWebhookCobrComSufixo(ctx context.Context, r gen.ReceberWebhookCobrComSufixoRequestObject) (gen.ReceberWebhookCobrComSufixoResponseObject, error) {
	if err := s.receberWebhookCobr(ctx, r.ContaId, r.Token, r.Body); err != nil {
		return nil, err
	}
	return gen.ReceberWebhookCobrComSufixo200Response{}, nil
}

func (s *Server) receberWebhookCobr(ctx context.Context, conta, token string, corpo *gen.ReceberWebhookCobrJSONBody) error {
	b, err := json.Marshal(corpo)
	if err != nil {
		return err
	}
	_, err = s.webhooks.ReceberCobr(ctx, conta, token, b)
	return err
}
