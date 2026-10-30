package httpapi

import (
	"context"
	"fmt"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// errSemCobr responde 501 quando o servidor foi montado sem cobrança recorrente.
var errSemCobr = fmt.Errorf("%w: a cobrança recorrente não está configurada nesta instância", ports.ErrNaoSuportado)

// CriarCobr cria a cobrança recorrente com txid gerado pelo Vintém.
func (s *Server) CriarCobr(ctx context.Context, r gen.CriarCobrRequestObject) (gen.CriarCobrResponseObject, error) {
	c, err := s.criarCobr(ctx, "", *r.Body)
	if err != nil {
		return nil, err
	}
	return gen.CriarCobr201JSONResponse(c), nil
}

// CriarCobrComTxid cria a cobrança recorrente com o txid do cliente.
func (s *Server) CriarCobrComTxid(ctx context.Context, r gen.CriarCobrComTxidRequestObject) (gen.CriarCobrComTxidResponseObject, error) {
	c, err := s.criarCobr(ctx, domain.Txid(r.Txid), *r.Body)
	if err != nil {
		return nil, err
	}
	return gen.CriarCobrComTxid201JSONResponse(c), nil
}

func (s *Server) criarCobr(ctx context.Context, txid domain.Txid, b gen.CobRSolicitada) (gen.CobRGerada, error) {
	if s.cobrs.Store == nil {
		return gen.CobRGerada{}, errSemCobr
	}
	valor, err := domain.ParseMoney(b.Valor.Original)
	if err != nil {
		return gen.CobRGerada{}, &app.ErroValidacao{Violacoes: []app.Violacao{{Propriedade: "valor.original", Razao: err.Error(), Valor: b.Valor.Original}}}
	}
	c := domain.CobrancaRecorrente{
		Txid: txid, Valor: valor, Vencimento: dataDe(b.Calendario.DataDeVencimento),
		AjusteDiaUtil: b.AjusteDiaUtil == nil || *b.AjusteDiaUtil, InfoAdicional: deref(b.InfoAdicional),
	}
	criada, err := s.cobrs.Criar(ctx, b.IdRec, c)
	if err != nil {
		return gen.CobRGerada{}, err
	}
	return cobrGen(criada), nil
}

// ConsultarCobr devolve a cobrança recorrente, se a API key enxergar a conta dela.
func (s *Server) ConsultarCobr(ctx context.Context, r gen.ConsultarCobrRequestObject) (gen.ConsultarCobrResponseObject, error) {
	if s.cobrs.Store == nil {
		return nil, errSemCobr
	}
	c, err := s.cobrs.Consultar(ctx, domain.Txid(r.Txid))
	if err != nil {
		return nil, err
	}
	return gen.ConsultarCobr200JSONResponse(cobrGen(c)), nil
}

// CancelarCobr leva a cobrança recorrente a CANCELADA: é a única revisão que a spec permite.
func (s *Server) CancelarCobr(ctx context.Context, r gen.CancelarCobrRequestObject) (gen.CancelarCobrResponseObject, error) {
	if s.cobrs.Store == nil {
		return nil, errSemCobr
	}
	c, err := s.cobrs.Cancelar(ctx, domain.Txid(r.Txid))
	if err != nil {
		return nil, err
	}
	return gen.CancelarCobr200JSONResponse(cobrGen(c)), nil
}

// RetentativaCobr pede uma nova data de liquidação.
func (s *Server) RetentativaCobr(ctx context.Context, r gen.RetentativaCobrRequestObject) (gen.RetentativaCobrResponseObject, error) {
	if s.cobrs.Store == nil {
		return nil, errSemCobr
	}
	c, err := s.cobrs.SolicitarRetentativa(ctx, domain.Txid(r.Txid), dataDe(r.Data))
	if err != nil {
		return nil, err
	}
	return gen.RetentativaCobr201JSONResponse(cobrGen(c)), nil
}

func cobrGen(c domain.CobrancaRecorrente) gen.CobRGerada {
	g := gen.CobRGerada{
		Txid: c.Txid.String(), IdRec: c.RecorrenciaID, Status: gen.CobRGeradaStatus(c.Status),
		AjusteDiaUtil: c.AjusteDiaUtil, Tentativas: make([]gen.CobRTentativa, len(c.Tentativas)),
	}
	g.Calendario.DataDeVencimento = openapi_types.Date{Time: c.Vencimento.Tempo()}
	g.Valor.Original = c.Valor.String()
	if c.InfoAdicional != "" {
		g.InfoAdicional = ptr(c.InfoAdicional)
	}
	if c.MotivoEncerramento != "" {
		g.MotivoEncerramento = ptr(c.MotivoEncerramento)
	}
	for i, t := range c.Tentativas {
		g.Tentativas[i] = gen.CobRTentativa{
			DataLiquidacao: openapi_types.Date{Time: t.DataLiquidacao.Tempo()},
			Tipo:           gen.CobRTentativaTipo(t.Tipo), Status: gen.CobRTentativaStatus(t.Status),
		}
		if t.EndToEndID != "" {
			g.Tentativas[i].EndToEndId = ptr(t.EndToEndID)
		}
		if t.MotivoRejeicao != "" {
			g.Tentativas[i].MotivoRejeicao = ptr(t.MotivoRejeicao)
		}
	}
	return g
}
