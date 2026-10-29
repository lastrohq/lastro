package sqlite

import (
	"context"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/store/sqlite/db"
	"github.com/lastrohq/lastro/internal/domain"
)

type idem struct{ q *db.Queries }

func (r idem) Buscar(ctx context.Context, dono, chave string) (domain.RegistroIdempotencia, error) {
	row, err := r.q.BuscarIdempotencia(ctx, db.BuscarIdempotenciaParams{Dono: dono, Chave: chave})
	if err != nil {
		return domain.RegistroIdempotencia{}, traduzir(err, "idempotência "+chave)
	}
	return domain.RegistroIdempotencia{
		Dono: row.Dono, Chave: row.Chave, Hash: row.Hash, Status: int(row.Status),
		ContentType: row.ContentType, Corpo: row.Corpo, CriadaEm: time.Unix(0, row.CriadaEm).UTC(),
	}, nil
}

func (r idem) Salvar(ctx context.Context, reg domain.RegistroIdempotencia) error {
	corpo := reg.Corpo
	if corpo == nil {
		corpo = []byte{}
	}
	err := r.q.SalvarIdempotencia(ctx, db.SalvarIdempotenciaParams{
		Dono: reg.Dono, Chave: reg.Chave, Hash: reg.Hash, Status: int64(reg.Status),
		ContentType: reg.ContentType, Corpo: corpo, CriadaEm: reg.CriadaEm.UnixNano(),
	})
	return traduzir(err, "idempotência "+reg.Chave)
}

func (r idem) Remover(ctx context.Context, dono, chave string) error {
	return traduzir(r.q.RemoverIdempotencia(ctx, db.RemoverIdempotenciaParams{Dono: dono, Chave: chave}), "idempotência "+chave)
}
