package app

import (
	"context"
	"errors"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// APIKeys gerencia e autentica as API keys (RN-60).
type APIKeys struct {
	Store ports.Store
	Clock ports.Clock
}

// Criar gera a chave e devolve o token em claro, que só aparece aqui.
func (s APIKeys) Criar(ctx context.Context, nome string, escopos []domain.Escopo, contaID string) (domain.APIKey, string, error) {
	k, token, err := domain.NovaAPIKey(nome, escopos, contaID, s.Clock.Now())
	if err != nil {
		return domain.APIKey{}, "", err
	}
	if err := s.Store.APIKeys().Criar(ctx, k); err != nil {
		return domain.APIKey{}, "", err
	}
	return k, token, nil
}

// Autenticar valida o token e devolve a chave. Qualquer falha vira
// ErrNaoAutenticado, sem dizer se o id existe.
func (s APIKeys) Autenticar(ctx context.Context, token string) (domain.APIKey, error) {
	id, segredo, err := domain.SepararToken(token)
	if err != nil {
		return domain.APIKey{}, err
	}
	k, err := s.Store.APIKeys().Buscar(ctx, id)
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return domain.APIKey{}, domain.ErrNaoAutenticado
	}
	if err != nil {
		return domain.APIKey{}, err
	}
	if err := k.Confere(segredo); err != nil {
		return domain.APIKey{}, err
	}
	return k, nil
}

// Listar devolve as chaves, sem segredo (o hash nem sai do Store para a CLI).
func (s APIKeys) Listar(ctx context.Context) ([]domain.APIKey, error) {
	return s.Store.APIKeys().Listar(ctx)
}

// Revogar invalida a chave imediatamente.
func (s APIKeys) Revogar(ctx context.Context, id string) error {
	return s.Store.APIKeys().Revogar(ctx, id, s.Clock.Now())
}
