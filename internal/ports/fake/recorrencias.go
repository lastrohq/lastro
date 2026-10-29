package fake

import (
	"context"
	"fmt"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Recorrencias devolve o repositório de recorrências.
func (s *Store) Recorrencias() ports.RecorrenciaRepo { return recorrencias{s} }

// Solicitacoes devolve o repositório de solicitações de confirmação.
func (s *Store) Solicitacoes() ports.SolicitacaoRepo { return solicitacoes{s} }

type recorrencias struct{ s *Store }

func (r recorrencias) Criar(_ context.Context, rec domain.Recorrencia) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.contas[rec.ContaID]; !ok {
		return fmt.Errorf("conta %s: %w", rec.ContaID, ports.ErrNaoEncontrado)
	}
	if _, ok := r.s.d.recs[rec.ID]; ok {
		return fmt.Errorf("recorrência %s: %w", rec.ID, ports.ErrJaExiste)
	}
	r.s.d.recs[rec.ID] = rec
	return nil
}

func (r recorrencias) Buscar(_ context.Context, id string) (domain.Recorrencia, error) {
	r.s.lock()
	defer r.s.unlock()
	rec, ok := r.s.d.recs[id]
	if !ok {
		return rec, fmt.Errorf("recorrência %s: %w", id, ports.ErrNaoEncontrado)
	}
	return rec, nil
}

func (r recorrencias) Atualizar(_ context.Context, rec *domain.Recorrencia) error {
	r.s.lock()
	defer r.s.unlock()
	atual, ok := r.s.d.recs[rec.ID]
	if !ok {
		return fmt.Errorf("recorrência %s: %w", rec.ID, ports.ErrNaoEncontrado)
	}
	if atual.Revisao != rec.Revisao {
		return fmt.Errorf("recorrência %s na revisão %d, não %d: %w", rec.ID, atual.Revisao, rec.Revisao, ports.ErrConflito)
	}
	atual.Status, atual.MotivoRejeicao = rec.Status, rec.MotivoRejeicao
	atual.Revisao++
	r.s.d.recs[rec.ID] = atual
	rec.Revisao = atual.Revisao
	return nil
}

type solicitacoes struct{ s *Store }

func (r solicitacoes) Criar(_ context.Context, sol domain.SolicitacaoRecorrencia) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.recs[sol.RecorrenciaID]; !ok {
		return fmt.Errorf("recorrência %s: %w", sol.RecorrenciaID, ports.ErrNaoEncontrado)
	}
	if _, ok := r.s.d.solics[sol.ID]; ok {
		return fmt.Errorf("solicitação %s: %w", sol.ID, ports.ErrJaExiste)
	}
	r.s.d.solics[sol.ID] = sol
	return nil
}

func (r solicitacoes) Buscar(_ context.Context, id string) (domain.SolicitacaoRecorrencia, error) {
	r.s.lock()
	defer r.s.unlock()
	sol, ok := r.s.d.solics[id]
	if !ok {
		return sol, fmt.Errorf("solicitação %s: %w", id, ports.ErrNaoEncontrado)
	}
	return sol, nil
}

func (r solicitacoes) AtualizarStatus(_ context.Context, id string, status domain.StatusSolicitacao) error {
	r.s.lock()
	defer r.s.unlock()
	sol, ok := r.s.d.solics[id]
	if !ok {
		return fmt.Errorf("solicitação %s: %w", id, ports.ErrNaoEncontrado)
	}
	sol.Status = status
	r.s.d.solics[id] = sol
	return nil
}
