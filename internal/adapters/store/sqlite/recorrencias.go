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

// Recorrencias devolve o repositório de recorrências de Pix Automático.
func (s *Store) Recorrencias() ports.RecorrenciaRepo { return recorrencias{s.q} }

// Solicitacoes devolve o repositório de solicitações de confirmação.
func (s *Store) Solicitacoes() ports.SolicitacaoRepo { return solicitacoes{s.q} }

type recorrencias struct{ q *db.Queries }

func (r recorrencias) Criar(ctx context.Context, rec domain.Recorrencia) error {
	p := db.CriarRecorrenciaParams{
		ID: rec.ID, ContaID: rec.ContaID, Status: string(rec.Status), Contrato: rec.Contrato, Objeto: rec.Objeto,
		DevedorDocumento: rec.Devedor.Documento, DevedorNome: rec.Devedor.Nome, DataInicial: rec.DataInicial.String(),
		Periodicidade: string(rec.Periodicidade), ValorFixo: int64(rec.ValorFixo), PisoValorMaximo: int64(rec.PisoValorMaximo),
		Politica: string(rec.Politica), MotivoRejeicao: rec.MotivoRejeicao, CriadaEm: rec.CriadaEm.UnixNano(), Revisao: int64(rec.Revisao),
	}
	if !rec.DataFinal.Zero() {
		p.DataFinal = sql.NullString{String: rec.DataFinal.String(), Valid: true}
	}
	err := r.q.CriarRecorrencia(ctx, p)
	if chaveEstrangeira(err) {
		return fmt.Errorf("conta %s: %w", rec.ContaID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, "recorrência "+rec.ID)
}

func (r recorrencias) Buscar(ctx context.Context, id string) (domain.Recorrencia, error) {
	row, err := r.q.BuscarRecorrencia(ctx, id)
	if err != nil {
		return domain.Recorrencia{}, traduzir(err, "recorrência "+id)
	}
	inicial, err := domain.ParseData(row.DataInicial)
	if err != nil {
		return domain.Recorrencia{}, fmt.Errorf("recorrência %s: data_inicial: %w", id, err)
	}
	rec := domain.Recorrencia{
		ID: row.ID, ContaID: row.ContaID, Status: domain.StatusRecorrencia(row.Status), Contrato: row.Contrato, Objeto: row.Objeto,
		Devedor: domain.Devedor{Documento: row.DevedorDocumento, Nome: row.DevedorNome}, DataInicial: inicial,
		Periodicidade: domain.Periodicidade(row.Periodicidade), ValorFixo: domain.Money(row.ValorFixo), PisoValorMaximo: domain.Money(row.PisoValorMaximo),
		Politica: domain.PoliticaRetentativa(row.Politica), MotivoRejeicao: row.MotivoRejeicao,
		CriadaEm: time.Unix(0, row.CriadaEm).UTC(), Revisao: int(row.Revisao),
	}
	if row.DataFinal.Valid {
		if rec.DataFinal, err = domain.ParseData(row.DataFinal.String); err != nil {
			return domain.Recorrencia{}, fmt.Errorf("recorrência %s: data_final: %w", id, err)
		}
	}
	return rec, nil
}

func (r recorrencias) Atualizar(ctx context.Context, rec *domain.Recorrencia) error {
	n, err := r.q.AtualizarRecorrencia(ctx, db.AtualizarRecorrenciaParams{
		Status: string(rec.Status), MotivoRejeicao: rec.MotivoRejeicao, ID: rec.ID, Revisao: int64(rec.Revisao),
	})
	if err != nil {
		return traduzir(err, "recorrência "+rec.ID)
	}
	if n == 0 {
		existe, err := r.q.ExisteRecorrencia(ctx, rec.ID)
		if err != nil {
			return traduzir(err, "recorrência "+rec.ID)
		}
		if existe == 0 {
			return fmt.Errorf("recorrência %s: %w", rec.ID, ports.ErrNaoEncontrado)
		}
		return fmt.Errorf("recorrência %s fora da revisão %d: %w", rec.ID, rec.Revisao, ports.ErrConflito)
	}
	rec.Revisao++
	return nil
}

type solicitacoes struct{ q *db.Queries }

func (r solicitacoes) Criar(ctx context.Context, s domain.SolicitacaoRecorrencia) error {
	err := r.q.CriarSolicitacao(ctx, db.CriarSolicitacaoParams{
		ID: s.ID, RecorrenciaID: s.RecorrenciaID, ContaID: s.ContaID, Status: string(s.Status),
		Ispb: s.Destinatario.ISPB, Agencia: s.Destinatario.Agencia, Conta: s.Destinatario.Conta,
		ExpiraEm: s.ExpiraEm.UnixNano(), CriadaEm: s.CriadaEm.UnixNano(),
	})
	if chaveEstrangeira(err) {
		return fmt.Errorf("recorrência %s: %w", s.RecorrenciaID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, "solicitação "+s.ID)
}

func (r solicitacoes) Buscar(ctx context.Context, id string) (domain.SolicitacaoRecorrencia, error) {
	row, err := r.q.BuscarSolicitacao(ctx, id)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, traduzir(err, "solicitação "+id)
	}
	return domain.SolicitacaoRecorrencia{
		ID: row.ID, RecorrenciaID: row.RecorrenciaID, ContaID: row.ContaID, Status: domain.StatusSolicitacao(row.Status),
		Destinatario: domain.ContaPagador{ISPB: row.Ispb, Agencia: row.Agencia, Conta: row.Conta},
		ExpiraEm:     time.Unix(0, row.ExpiraEm).UTC(), CriadaEm: time.Unix(0, row.CriadaEm).UTC(),
	}, nil
}

func (r solicitacoes) AtualizarStatus(ctx context.Context, id string, status domain.StatusSolicitacao) error {
	n, err := r.q.AtualizarStatusSolicitacao(ctx, db.AtualizarStatusSolicitacaoParams{Status: string(status), ID: id})
	if err != nil {
		return traduzir(err, "solicitação "+id)
	}
	if n == 0 {
		return fmt.Errorf("solicitação %s: %w", id, ports.ErrNaoEncontrado)
	}
	return nil
}
