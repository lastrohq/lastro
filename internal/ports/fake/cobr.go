package fake

import (
	"context"
	"fmt"
	"slices"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// CobrancasRecorrentes devolve o repositório de cobranças recorrentes.
func (s *Store) CobrancasRecorrentes() ports.CobrancaRecorrenteRepo { return cobrs{s} }

type cobrs struct{ s *Store }

func (r cobrs) Criar(_ context.Context, c domain.CobrancaRecorrente) error {
	r.s.lock()
	defer r.s.unlock()
	if r.s.d.txidUsado(c.Txid) {
		return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrJaExiste)
	}
	if _, ok := r.s.d.recs[c.RecorrenciaID]; !ok {
		return fmt.Errorf("recorrência %s: %w", c.RecorrenciaID, ports.ErrNaoEncontrado)
	}
	c.Tentativas = slices.Clone(c.Tentativas)
	r.s.d.cobrs[c.Txid] = c
	return nil
}

func (r cobrs) BuscarPorTxid(_ context.Context, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.cobrs[txid]
	if !ok {
		return c, fmt.Errorf("cobrança recorrente %s: %w", txid, ports.ErrNaoEncontrado)
	}
	c.Tentativas = slices.Clone(c.Tentativas)
	return c, nil
}

func (r cobrs) Atualizar(_ context.Context, c *domain.CobrancaRecorrente) error {
	r.s.lock()
	defer r.s.unlock()
	atual, ok := r.s.d.cobrs[c.Txid]
	if !ok {
		return fmt.Errorf("cobrança recorrente %s: %w", c.Txid, ports.ErrNaoEncontrado)
	}
	if atual.Revisao != c.Revisao {
		return fmt.Errorf("cobrança recorrente %s na revisão %d, não %d: %w", c.Txid, atual.Revisao, c.Revisao, ports.ErrConflito)
	}
	atual.Status, atual.Tentativas, atual.MotivoEncerramento = c.Status, slices.Clone(c.Tentativas), c.MotivoEncerramento
	atual.Revisao++
	r.s.d.cobrs[c.Txid] = atual
	c.Revisao = atual.Revisao
	return nil
}

func (r cobrs) DaRecorrencia(_ context.Context, idRec string) ([]domain.CobrancaRecorrente, error) {
	r.s.lock()
	defer r.s.unlock()
	var out []domain.CobrancaRecorrente
	for _, c := range r.s.d.cobrs {
		if c.RecorrenciaID == idRec {
			c.Tentativas = slices.Clone(c.Tentativas)
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b domain.CobrancaRecorrente) int {
		if n := a.Vencimento.Tempo().Compare(b.Vencimento.Tempo()); n != 0 {
			return n
		}
		return a.CriadaEm.Compare(b.CriadaEm)
	})
	return out, nil
}

func (r cobrs) Abertas(_ context.Context, contaID string) ([]domain.CobrancaRecorrente, error) {
	r.s.lock()
	defer r.s.unlock()
	var out []domain.CobrancaRecorrente
	for _, c := range r.s.d.cobrs {
		if c.ContaID == contaID && !domain.TransicoesCobrancaRecorrente.Terminal(c.Status) {
			c.Tentativas = slices.Clone(c.Tentativas)
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b domain.CobrancaRecorrente) int { return a.CriadaEm.Compare(b.CriadaEm) })
	return out, nil
}
