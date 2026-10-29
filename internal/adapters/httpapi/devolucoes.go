package httpapi

import (
	"context"
	"fmt"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// errSemDevolucoes responde 501 quando o servidor foi montado sem devoluções.
var errSemDevolucoes = fmt.Errorf("%w: as devoluções não estão configuradas nesta instância", ports.ErrNaoSuportado)

// SolicitarDevolucao devolve o Pix total ou parcialmente (RN-21).
func (s *Server) SolicitarDevolucao(ctx context.Context, r gen.SolicitarDevolucaoRequestObject) (gen.SolicitarDevolucaoResponseObject, error) {
	if s.devolucoes.Store == nil {
		return nil, errSemDevolucoes
	}
	valor, err := domain.ParseMoney(r.Body.Valor)
	if err != nil {
		return nil, &app.ErroValidacao{Violacoes: []app.Violacao{{Propriedade: "valor", Razao: err.Error(), Valor: r.Body.Valor}}}
	}
	d := domain.Devolucao{EndToEndID: r.E2eid, ID: r.Id, Valor: valor, Descricao: deref(r.Body.Descricao)}
	if r.Body.Natureza != nil {
		d.Natureza = domain.NaturezaDevolucao(*r.Body.Natureza)
	}
	feita, err := s.devolucoes.Solicitar(ctx, d)
	if err != nil {
		return nil, err
	}
	return gen.SolicitarDevolucao201JSONResponse(devolucaoGen(feita)), nil
}

// ConsultarDevolucao devolve a devolução, se a API key enxergar a conta dela.
func (s *Server) ConsultarDevolucao(ctx context.Context, r gen.ConsultarDevolucaoRequestObject) (gen.ConsultarDevolucaoResponseObject, error) {
	if s.devolucoes.Store == nil {
		return nil, errSemDevolucoes
	}
	d, err := s.devolucoes.Consultar(ctx, r.E2eid, r.Id)
	if err != nil {
		return nil, err
	}
	return gen.ConsultarDevolucao200JSONResponse(devolucaoGen(d)), nil
}

func devolucaoGen(d domain.Devolucao) gen.Devolucao {
	g := gen.Devolucao{Id: d.ID, Valor: d.Valor.String(), Natureza: gen.DevolucaoNatureza(d.Natureza), Status: gen.DevolucaoStatus(d.Status)}
	g.Horario.Solicitacao = d.CriadaEm
	if !d.LiquidadaEm.IsZero() {
		g.Horario.Liquidacao = ptr(d.LiquidadaEm)
	}
	if d.RtrID != "" {
		g.RtrId = ptr(d.RtrID)
	}
	if d.Descricao != "" {
		g.Descricao = ptr(d.Descricao)
	}
	if d.Motivo != "" {
		g.Motivo = ptr(d.Motivo)
	}
	return g
}
