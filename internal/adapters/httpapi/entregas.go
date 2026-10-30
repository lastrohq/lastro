package httpapi

import (
	"context"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/domain"
)

// ListarEntregas lista as entregas de webhook de saída num status (padrão: DLQ).
func (s *Server) ListarEntregas(ctx context.Context, r gen.ListarEntregasRequestObject) (gen.ListarEntregasResponseObject, error) {
	status, limite := domain.EntregaDLQ, 100
	if r.Params.Status != nil {
		status = domain.StatusEntrega(*r.Params.Status)
	}
	if r.Params.Limite != nil {
		limite = *r.Params.Limite
	}
	lista, err := s.saida.Entregas(ctx, status, limite)
	if err != nil {
		return nil, err
	}
	out := gen.ListarEntregas200JSONResponse{Entregas: make([]gen.EntregaWebhook, len(lista))}
	for i, e := range lista {
		g := gen.EntregaWebhook{
			EventoId: e.EventoID, DestinoId: e.DestinoID, ContaId: e.ContaID, Tipo: string(e.Tipo),
			Status: gen.EntregaWebhookStatus(e.Status), Tentativas: e.Tentativas,
			CriadaEm: e.CriadaEm, ProximaEm: e.ProximaEm, AtualizadaEm: e.AtualizadaEm,
		}
		if e.UltimoHTTP != 0 {
			g.UltimoHttp = &e.UltimoHTTP
		}
		if e.UltimoErro != "" {
			g.UltimoErro = &e.UltimoErro
		}
		out.Entregas[i] = g
	}
	return out, nil
}

// ReenviarEntregas devolve à fila as entregas do evento que estão na DLQ.
func (s *Server) ReenviarEntregas(ctx context.Context, r gen.ReenviarEntregasRequestObject) (gen.ReenviarEntregasResponseObject, error) {
	n, err := s.saida.ReenviarVisivel(ctx, r.EventoId)
	if err != nil {
		return nil, err
	}
	return gen.ReenviarEntregas200JSONResponse{Reenviadas: n}, nil
}
