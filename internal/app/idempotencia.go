package app

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

var (
	// ErrIdempotenciaDivergente indica a mesma Idempotency-Key com outro pedido.
	ErrIdempotenciaDivergente = errors.New("Idempotency-Key já usada com outra requisição")
	// ErrIdempotenciaEmAndamento indica que a primeira requisição com a chave ainda não respondeu.
	ErrIdempotenciaEmAndamento = errors.New("já existe uma requisição em andamento com essa Idempotency-Key")
)

const (
	// ValidadeIdempotencia é por quanto tempo a resposta fica guardada (RN-11).
	ValidadeIdempotencia = 24 * time.Hour
	// ReservaAbandonada é quanto tempo uma requisição "em andamento" segura a
	// chave. Passou disso, o processo que a reservou provavelmente caiu.
	ReservaAbandonada = 2 * time.Minute
)

// RespostaGuardada é o que se devolve ao repetir uma requisição.
type RespostaGuardada struct {
	Status      int
	ContentType string
	Corpo       []byte
}

// Idempotencia implementa o Idempotency-Key (RN-11).
type Idempotencia struct {
	Store ports.Store
	Clock ports.Clock
}

// Iniciar reserva a chave para o pedido. Devolve a resposta guardada se a
// mesma requisição já foi respondida, ou nil se a requisição deve seguir.
func (s Idempotencia) Iniciar(ctx context.Context, dono, chave string, hash []byte) (*RespostaGuardada, error) {
	var guardada *RespostaGuardada
	err := s.Store.WithTx(ctx, func(tx ports.Store) error {
		agora := s.Clock.Now()
		reg, err := tx.Idempotencia().Buscar(ctx, dono, chave)
		switch {
		case errors.Is(err, ports.ErrNaoEncontrado):
		case err != nil:
			return err
		case agora.Sub(reg.CriadaEm) >= ValidadeIdempotencia:
			// Expirou: a chave pode ser reutilizada.
		case !bytes.Equal(reg.Hash, hash):
			return ErrIdempotenciaDivergente
		case reg.Status == 0 && agora.Sub(reg.CriadaEm) < ReservaAbandonada:
			return ErrIdempotenciaEmAndamento
		case reg.Status != 0:
			guardada = &RespostaGuardada{Status: reg.Status, ContentType: reg.ContentType, Corpo: reg.Corpo}
			return nil
		}
		return tx.Idempotencia().Salvar(ctx, domain.RegistroIdempotencia{Dono: dono, Chave: chave, Hash: hash, CriadaEm: agora})
	})
	return guardada, err
}

// Concluir guarda a resposta. Erro 5xx não é guardado: libera a chave para o
// cliente tentar de novo, que é justamente para isso que ele mandou a chave.
func (s Idempotencia) Concluir(ctx context.Context, dono, chave string, hash []byte, r RespostaGuardada) error {
	if r.Status >= 500 {
		return s.Store.Idempotencia().Remover(ctx, dono, chave)
	}
	return s.Store.Idempotencia().Salvar(ctx, domain.RegistroIdempotencia{
		Dono: dono, Chave: chave, Hash: hash, Status: r.Status, ContentType: r.ContentType, Corpo: r.Corpo, CriadaEm: s.Clock.Now(),
	})
}
