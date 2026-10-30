package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/store/sqlite/db"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

func (r outbox) NaoDistribuidos(ctx context.Context, limite int) ([]ports.EventoArmazenado, error) {
	rows, err := r.q.NaoDistribuidos(ctx, int64(limite))
	if err != nil {
		return nil, traduzir(err, "outbox")
	}
	out := make([]ports.EventoArmazenado, len(rows))
	for i, row := range rows {
		out[i] = ports.EventoArmazenado{ID: row.ID, Tipo: domain.TipoEvento(row.Tipo), ContaID: row.ContaID, OcorridoEm: time.Unix(0, row.OcorridoEm).UTC(), Dados: []byte(row.Dados)}
	}
	return out, nil
}

func (r outbox) MarcarDistribuido(ctx context.Context, id string) error {
	return traduzir(r.q.MarcarDistribuido(ctx, id), "evento "+id)
}

type destinos struct{ q *db.Queries }

func (r destinos) Criar(ctx context.Context, d domain.Destino) error {
	err := r.q.CriarDestino(ctx, db.CriarDestinoParams{
		ID: d.ID, ContaID: sql.NullString{String: d.ContaID, Valid: d.ContaID != ""}, Url: d.URL,
		SegredoCifrado: d.SegredoCifrado, CriadoEm: d.CriadoEm.UnixNano(),
	})
	if chaveEstrangeira(err) {
		return fmt.Errorf("conta %s: %w", d.ContaID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, "destino "+d.ID)
}

func paraDestino(r db.DestinosWebhook) domain.Destino {
	return domain.Destino{ID: r.ID, ContaID: r.ContaID.String, URL: r.Url, SegredoCifrado: r.SegredoCifrado, CriadoEm: time.Unix(0, r.CriadoEm).UTC()}
}

func (r destinos) Listar(ctx context.Context) ([]domain.Destino, error) {
	rows, err := r.q.ListarDestinos(ctx)
	if err != nil {
		return nil, traduzir(err, "destinos")
	}
	out := make([]domain.Destino, len(rows))
	for i, row := range rows {
		out[i] = paraDestino(row)
	}
	return out, nil
}

func (r destinos) DaConta(ctx context.Context, contaID string) ([]domain.Destino, error) {
	rows, err := r.q.DestinosDaConta(ctx, sql.NullString{String: contaID, Valid: true})
	if err != nil {
		return nil, traduzir(err, "destinos da conta "+contaID)
	}
	out := make([]domain.Destino, len(rows))
	for i, row := range rows {
		out[i] = paraDestino(row)
	}
	return out, nil
}

func (r destinos) AtualizarSegredo(ctx context.Context, id string, cifrado []byte) error {
	n, err := r.q.AtualizarSegredoDestino(ctx, db.AtualizarSegredoDestinoParams{SegredoCifrado: cifrado, ID: id})
	if err != nil {
		return traduzir(err, "destino "+id)
	}
	if n == 0 {
		return fmt.Errorf("destino %s: %w", id, ports.ErrNaoEncontrado)
	}
	return nil
}

func (r destinos) Remover(ctx context.Context, id string) error {
	n, err := r.q.RemoverDestino(ctx, id)
	if err != nil {
		return traduzir(err, "destino "+id)
	}
	if n == 0 {
		return fmt.Errorf("destino %s: %w", id, ports.ErrNaoEncontrado)
	}
	return nil
}

type entregas struct{ q *db.Queries }

func (r entregas) Criar(ctx context.Context, e domain.Entrega) error {
	err := r.q.CriarEntrega(ctx, db.CriarEntregaParams{
		EventoID: e.EventoID, DestinoID: e.DestinoID, ContaID: e.ContaID, Tipo: string(e.Tipo), Corpo: e.Corpo,
		CriadaEm: e.CriadaEm.UnixNano(), Status: string(e.Status), Tentativas: int64(e.Tentativas), ProximaEm: e.ProximaEm.UnixNano(),
		UltimoHttp: int64(e.UltimoHTTP), UltimoErro: e.UltimoErro, AtualizadaEm: e.AtualizadaEm.UnixNano(),
	})
	return traduzir(err, "entrega "+e.EventoID)
}

func paraEntrega(r db.EntregasWebhook) domain.Entrega {
	return domain.Entrega{
		EventoID: r.EventoID, DestinoID: r.DestinoID, ContaID: r.ContaID, Tipo: domain.TipoEvento(r.Tipo), Corpo: r.Corpo,
		CriadaEm: time.Unix(0, r.CriadaEm).UTC(), Status: domain.StatusEntrega(r.Status), Tentativas: int(r.Tentativas),
		ProximaEm: time.Unix(0, r.ProximaEm).UTC(), UltimoHTTP: int(r.UltimoHttp), UltimoErro: r.UltimoErro,
		AtualizadaEm: time.Unix(0, r.AtualizadaEm).UTC(),
	}
}

func listaEntregas(rows []db.EntregasWebhook) []domain.Entrega {
	out := make([]domain.Entrega, len(rows))
	for i, row := range rows {
		out[i] = paraEntrega(row)
	}
	return out
}

func (r entregas) Proximas(ctx context.Context, agora time.Time, limite int) ([]domain.Entrega, error) {
	rows, err := r.q.ProximasEntregas(ctx, db.ProximasEntregasParams{ProximaEm: agora.UnixNano(), Limit: int64(limite)})
	if err != nil {
		return nil, traduzir(err, "entregas")
	}
	return listaEntregas(rows), nil
}

func (r entregas) Atualizar(ctx context.Context, e domain.Entrega) error {
	n, err := r.q.AtualizarEntrega(ctx, db.AtualizarEntregaParams{
		Status: string(e.Status), Tentativas: int64(e.Tentativas), ProximaEm: e.ProximaEm.UnixNano(), UltimoHttp: int64(e.UltimoHTTP),
		UltimoErro: e.UltimoErro, AtualizadaEm: e.AtualizadaEm.UnixNano(), EventoID: e.EventoID, DestinoID: e.DestinoID,
	})
	if err != nil {
		return traduzir(err, "entrega "+e.EventoID)
	}
	if n == 0 {
		return fmt.Errorf("entrega %s/%s: %w", e.EventoID, e.DestinoID, ports.ErrNaoEncontrado)
	}
	return nil
}

func (r entregas) Listar(ctx context.Context, status domain.StatusEntrega, limite int) ([]domain.Entrega, error) {
	rows, err := r.q.ListarEntregas(ctx, db.ListarEntregasParams{Status: string(status), Limit: int64(limite)})
	if err != nil {
		return nil, traduzir(err, "entregas")
	}
	return listaEntregas(rows), nil
}

func (r entregas) Reenviar(ctx context.Context, eventoID string, agora time.Time) (int, error) {
	n, err := r.q.ReenviarEntregas(ctx, db.ReenviarEntregasParams{Agora: agora.UnixNano(), EventoID: eventoID})
	return int(n), traduzir(err, "entregas do evento "+eventoID)
}
