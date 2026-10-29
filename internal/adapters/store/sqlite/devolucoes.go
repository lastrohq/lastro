package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/store/sqlite/db"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Devolucoes devolve o repositório de devoluções.
func (s *Store) Devolucoes() ports.DevolucaoRepo { return devolucoes{s.q} }

type devolucoes struct{ q *db.Queries }

func nomeDevolucao(e2e, id string) string { return "devolução " + id + " do Pix " + e2e }

func instanteOpcional(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixNano(), Valid: true}
}

func (r devolucoes) Criar(ctx context.Context, d domain.Devolucao) error {
	err := r.q.CriarDevolucao(ctx, db.CriarDevolucaoParams{
		EndToEndID: d.EndToEndID, ID: d.ID, ContaID: d.ContaID, Valor: int64(d.Valor), Natureza: string(d.Natureza),
		Descricao: d.Descricao, Status: string(d.Status), Motivo: d.Motivo, RtrID: d.RtrID,
		CriadaEm: d.CriadaEm.UnixNano(), LiquidadaEm: instanteOpcional(d.LiquidadaEm), Revisao: int64(d.Revisao),
	})
	if chaveEstrangeira(err) {
		return fmt.Errorf("pix %s: %w", d.EndToEndID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, nomeDevolucao(d.EndToEndID, d.ID))
}

func paraDevolucao(row db.Devolucao) domain.Devolucao {
	d := domain.Devolucao{
		ID: row.ID, EndToEndID: row.EndToEndID, ContaID: row.ContaID, Valor: domain.Money(row.Valor),
		Natureza: domain.NaturezaDevolucao(row.Natureza), Descricao: row.Descricao, Status: domain.StatusDevolucao(row.Status),
		Motivo: row.Motivo, RtrID: row.RtrID, CriadaEm: time.Unix(0, row.CriadaEm).UTC(), Revisao: int(row.Revisao),
	}
	if row.LiquidadaEm.Valid {
		d.LiquidadaEm = time.Unix(0, row.LiquidadaEm.Int64).UTC()
	}
	return d
}

func paraDevolucoes(rows []db.Devolucao) []domain.Devolucao {
	out := make([]domain.Devolucao, len(rows))
	for i, row := range rows {
		out[i] = paraDevolucao(row)
	}
	return out
}

func (r devolucoes) Buscar(ctx context.Context, e2e, id string) (domain.Devolucao, error) {
	row, err := r.q.BuscarDevolucao(ctx, db.BuscarDevolucaoParams{EndToEndID: e2e, ID: id})
	if err != nil {
		return domain.Devolucao{}, traduzir(err, nomeDevolucao(e2e, id))
	}
	return paraDevolucao(row), nil
}

func (r devolucoes) DoPix(ctx context.Context, e2e string) ([]domain.Devolucao, error) {
	rows, err := r.q.DevolucoesDoPix(ctx, e2e)
	if err != nil {
		return nil, traduzir(err, "devoluções do Pix "+e2e)
	}
	return paraDevolucoes(rows), nil
}

func (r devolucoes) Atualizar(ctx context.Context, d *domain.Devolucao) error {
	n, err := r.q.AtualizarDevolucao(ctx, db.AtualizarDevolucaoParams{
		Status: string(d.Status), Motivo: d.Motivo, RtrID: d.RtrID, LiquidadaEm: instanteOpcional(d.LiquidadaEm),
		EndToEndID: d.EndToEndID, ID: d.ID, Revisao: int64(d.Revisao),
	})
	if err != nil {
		return traduzir(err, nomeDevolucao(d.EndToEndID, d.ID))
	}
	if n == 0 {
		if _, err := r.Buscar(ctx, d.EndToEndID, d.ID); err != nil {
			return err
		}
		return fmt.Errorf("%s fora da revisão %d: %w", nomeDevolucao(d.EndToEndID, d.ID), d.Revisao, ports.ErrConflito)
	}
	d.Revisao++
	return nil
}

func (r devolucoes) EmProcessamento(ctx context.Context, contaID string) ([]domain.Devolucao, error) {
	rows, err := r.q.DevolucoesEmProcessamento(ctx, contaID)
	if err != nil {
		return nil, traduzir(err, "devoluções em processamento da conta "+contaID)
	}
	return paraDevolucoes(rows), nil
}
