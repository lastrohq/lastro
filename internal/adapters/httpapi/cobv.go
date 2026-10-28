package httpapi

import (
	"context"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
)

// Cobrança com vencimento: responde 501 até a #15.

// ListarCobvs lista cobranças com vencimento.
func (s *Server) ListarCobvs(context.Context, gen.ListarCobvsRequestObject) (gen.ListarCobvsResponseObject, error) {
	return nil, errNaoImplementado
}

// ConsultarCobv consulta uma cobrança com vencimento.
func (s *Server) ConsultarCobv(context.Context, gen.ConsultarCobvRequestObject) (gen.ConsultarCobvResponseObject, error) {
	return nil, errNaoImplementado
}

// RevisarCobv revisa uma cobrança com vencimento.
func (s *Server) RevisarCobv(context.Context, gen.RevisarCobvRequestObject) (gen.RevisarCobvResponseObject, error) {
	return nil, errNaoImplementado
}

// CriarCobv cria cobrança com vencimento.
func (s *Server) CriarCobv(context.Context, gen.CriarCobvRequestObject) (gen.CriarCobvResponseObject, error) {
	return nil, errNaoImplementado
}
