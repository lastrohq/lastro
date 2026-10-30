package fake

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// Devolucoes devolve o repositório de devoluções.
func (s *Store) Devolucoes() ports.DevolucaoRepo { return devolucoes{s} }

type devolucoes struct{ s *Store }

func (r devolucoes) Criar(_ context.Context, d domain.Devolucao) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.pix[d.EndToEndID]; !ok {
		return fmt.Errorf("pix %s: %w", d.EndToEndID, ports.ErrNaoEncontrado)
	}
	k := [2]string{d.EndToEndID, d.ID}
	if _, ja := r.s.d.devs[k]; ja {
		return fmt.Errorf("devolução %s do Pix %s: %w", d.ID, d.EndToEndID, ports.ErrJaExiste)
	}
	r.s.d.devs[k] = d
	return nil
}

func (r devolucoes) Buscar(_ context.Context, e2e, id string) (domain.Devolucao, error) {
	r.s.lock()
	defer r.s.unlock()
	d, ok := r.s.d.devs[[2]string{e2e, id}]
	if !ok {
		return d, fmt.Errorf("devolução %s do Pix %s: %w", id, e2e, ports.ErrNaoEncontrado)
	}
	return d, nil
}

func (r devolucoes) filtrar(f func(domain.Devolucao) bool) []domain.Devolucao {
	r.s.lock()
	defer r.s.unlock()
	var out []domain.Devolucao
	for _, d := range r.s.d.devs {
		if f(d) {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(a, b domain.Devolucao) int {
		return cmp.Or(a.CriadaEm.Compare(b.CriadaEm), cmp.Compare(a.EndToEndID, b.EndToEndID), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func (r devolucoes) DoPix(_ context.Context, e2e string) ([]domain.Devolucao, error) {
	return r.filtrar(func(d domain.Devolucao) bool { return d.EndToEndID == e2e }), nil
}

func (r devolucoes) Atualizar(_ context.Context, d *domain.Devolucao) error {
	r.s.lock()
	defer r.s.unlock()
	k := [2]string{d.EndToEndID, d.ID}
	atual, ok := r.s.d.devs[k]
	if !ok {
		return fmt.Errorf("devolução %s do Pix %s: %w", d.ID, d.EndToEndID, ports.ErrNaoEncontrado)
	}
	if atual.Revisao != d.Revisao {
		return fmt.Errorf("devolução %s do Pix %s na revisão %d, não %d: %w", d.ID, d.EndToEndID, atual.Revisao, d.Revisao, ports.ErrConflito)
	}
	atual.Status, atual.Motivo, atual.RtrID, atual.LiquidadaEm = d.Status, d.Motivo, d.RtrID, d.LiquidadaEm
	atual.Revisao++
	r.s.d.devs[k] = atual
	d.Revisao = atual.Revisao
	return nil
}

func (r devolucoes) EmProcessamento(_ context.Context, contaID string) ([]domain.Devolucao, error) {
	return r.filtrar(func(d domain.Devolucao) bool {
		return d.ContaID == contaID && d.Status == domain.DevolucaoEmProcessamento
	}), nil
}
