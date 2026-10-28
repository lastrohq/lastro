package httpapi

import (
	"context"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
)

// Rotas do contrato ainda sem caso de uso: respondem 501 até as issues
// de cob (#14) e cobv (#15) chegarem.

// ListarCobs lista cobranças imediatas.
func (s *Server) ListarCobs(context.Context, gen.ListarCobsRequestObject) (gen.ListarCobsResponseObject, error) {
	return nil, errNaoImplementado
}

// CriarCob cria cobrança imediata com txid gerado pelo Lastro.
func (s *Server) CriarCob(context.Context, gen.CriarCobRequestObject) (gen.CriarCobResponseObject, error) {
	return nil, errNaoImplementado
}

// ConsultarCob consulta uma cobrança imediata.
func (s *Server) ConsultarCob(context.Context, gen.ConsultarCobRequestObject) (gen.ConsultarCobResponseObject, error) {
	return nil, errNaoImplementado
}

// RevisarCob revisa uma cobrança imediata.
func (s *Server) RevisarCob(context.Context, gen.RevisarCobRequestObject) (gen.RevisarCobResponseObject, error) {
	return nil, errNaoImplementado
}

// CriarCobComTxid cria cobrança imediata com txid do cliente.
func (s *Server) CriarCobComTxid(context.Context, gen.CriarCobComTxidRequestObject) (gen.CriarCobComTxidResponseObject, error) {
	return nil, errNaoImplementado
}

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
