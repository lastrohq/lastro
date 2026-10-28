// Package sqlite implementa ports.Store sobre SQLite em Go puro, sem CGO (ADR-002).
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"net/url"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registra o driver "sqlite"

	"github.com/lastrohq/lastro/internal/adapters/store/sqlite/db"
	"github.com/lastrohq/lastro/internal/ports"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store é o ports.Store em SQLite.
type Store struct {
	db *sql.DB
	q  *db.Queries
	tx *sql.Tx
}

var _ ports.Store = (*Store)(nil)

// Open abre (ou cria) o banco em path, aplica as migrations pendentes e
// devolve o Store. WAL, busy_timeout e foreign_keys ficam ligados.
func Open(ctx context.Context, path string) (*Store, error) {
	q := url.Values{}
	for _, p := range []string{"journal_mode(WAL)", "busy_timeout(5000)", "foreign_keys(1)", "synchronous(NORMAL)"} {
		q.Add("_pragma", p)
	}
	q.Set("_txlock", "immediate")
	conn, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("abrindo sqlite: %w", err)
	}
	if err := migrar(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Store{db: conn, q: db.New(conn)}, nil
}

func migrar(ctx context.Context, conn *sql.DB) error {
	p, err := goose.NewProvider(goose.DialectSQLite3, conn, mustSub(migrations, "migrations"))
	if err != nil {
		return fmt.Errorf("preparando migrations: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("aplicando migrations: %w", err)
	}
	return nil
}

// Close fecha o banco.
func (s *Store) Close() error { return s.db.Close() }

// WithTx executa fn numa transação IMMEDIATE; erro em fn faz rollback.
func (s *Store) WithTx(ctx context.Context, fn func(ports.Store) error) (err error) {
	if s.tx != nil {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciando transação: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(&Store{db: s.db, q: s.q.WithTx(tx), tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}

// Contas devolve o repositório de contas.
func (s *Store) Contas() ports.ContaRepo { return contas{s.q} }

// Cobrancas devolve o repositório de cobranças.
func (s *Store) Cobrancas() ports.CobrancaRepo { return cobrancas{s.q} }

// APIKeys devolve o repositório de API keys.
func (s *Store) APIKeys() ports.APIKeyRepo { return apikeys{s.q} }

// Outbox devolve o repositório do outbox.
func (s *Store) Outbox() ports.OutboxRepo { return outbox{s.q} }

// traduzir converte erros do SQLite nos erros do port.
func traduzir(err error, oque string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", oque, ports.ErrNaoEncontrado)
	}
	if unico(err) {
		return fmt.Errorf("%s: %w", oque, ports.ErrJaExiste)
	}
	return fmt.Errorf("%s: %w", oque, err)
}
