package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/store/sqlite/db"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

type apikeys struct{ q *db.Queries }

func (r apikeys) Criar(ctx context.Context, k domain.APIKey) error {
	escopos := make([]string, len(k.Escopos))
	for i, e := range k.Escopos {
		escopos[i] = string(e)
	}
	err := r.q.CriarAPIKey(ctx, db.CriarAPIKeyParams{
		ID: k.ID, Nome: k.Nome, Hash: k.Hash, Escopos: strings.Join(escopos, " "),
		ContaID: sql.NullString{String: k.ContaID, Valid: k.ContaID != ""}, CriadaEm: k.CriadaEm.UnixNano(),
	})
	if chaveEstrangeira(err) {
		return fmt.Errorf("conta %s: %w", k.ContaID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, "api key "+k.ID)
}

func (r apikeys) Buscar(ctx context.Context, id string) (domain.APIKey, error) {
	row, err := r.q.BuscarAPIKey(ctx, id)
	if err != nil {
		return domain.APIKey{}, traduzir(err, "api key "+id)
	}
	return paraAPIKey(row), nil
}

func (r apikeys) Listar(ctx context.Context) ([]domain.APIKey, error) {
	rows, err := r.q.ListarAPIKeys(ctx)
	if err != nil {
		return nil, traduzir(err, "api keys")
	}
	out := make([]domain.APIKey, len(rows))
	for i, row := range rows {
		out[i] = paraAPIKey(row)
	}
	return out, nil
}

func (r apikeys) Revogar(ctx context.Context, id string, quando time.Time) error {
	n, err := r.q.RevogarAPIKey(ctx, db.RevogarAPIKeyParams{Quando: sql.NullInt64{Int64: quando.UnixNano(), Valid: true}, ID: id})
	if err != nil {
		return traduzir(err, "api key "+id)
	}
	if n == 0 {
		return fmt.Errorf("api key %s: %w", id, ports.ErrNaoEncontrado)
	}
	return nil
}

func paraAPIKey(r db.ApiKey) domain.APIKey {
	k := domain.APIKey{ID: r.ID, Nome: r.Nome, Hash: r.Hash, ContaID: r.ContaID.String, CriadaEm: time.Unix(0, r.CriadaEm).UTC()}
	for _, e := range strings.Fields(r.Escopos) {
		k.Escopos = append(k.Escopos, domain.Escopo(e))
	}
	if r.RevogadaEm.Valid {
		t := time.Unix(0, r.RevogadaEm.Int64).UTC()
		k.RevogadaEm = &t
	}
	return k
}
