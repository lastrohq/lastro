// Package app contém os casos de uso que orquestram domínio e ports.
package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Contas gerencia as contas recebedoras e as credenciais delas (RN-03, RN-61).
type Contas struct {
	Store ports.Store
	Vault ports.Vault
	PSPs  ports.Registry
}

// Criar valida a conta, confere se o PSP existe e grava a conta com as
// credenciais cifradas, numa transação só. Conta nasce ativa.
func (s Contas) Criar(ctx context.Context, c domain.Conta, cred domain.Credenciais) error {
	c.Ativa = true
	if err := c.Validar(); err != nil {
		return err
	}
	if _, err := s.PSPs.PSP(c.PSP); err != nil {
		return err
	}
	plano, err := json.Marshal(cred) //nolint:gosec // G117: serializado só para ser cifrado na linha seguinte
	if err != nil {
		return fmt.Errorf("serializando credenciais: %w", err)
	}
	cifrado, err := s.Vault.Cifrar(ctx, plano)
	if err != nil {
		return fmt.Errorf("cifrando credenciais: %w", err)
	}
	return s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Contas().Criar(ctx, c); err != nil {
			return err
		}
		return tx.Contas().SalvarCredenciais(ctx, c.ID, cifrado)
	})
}

// Desativar impede novas cobranças na conta; as existentes continuam valendo.
func (s Contas) Desativar(ctx context.Context, id string) error {
	return s.Store.WithTx(ctx, func(tx ports.Store) error {
		c, err := tx.Contas().Buscar(ctx, id)
		if err != nil {
			return err
		}
		c.Ativa = false
		return tx.Contas().Atualizar(ctx, c)
	})
}

// Buscar devolve uma conta, sem credenciais.
func (s Contas) Buscar(ctx context.Context, id string) (domain.Conta, error) {
	return s.Store.Contas().Buscar(ctx, id)
}

// Listar devolve as contas, sem credenciais.
func (s Contas) Listar(ctx context.Context) ([]domain.Conta, error) {
	return s.Store.Contas().Listar(ctx)
}

// Credenciais decifra as credenciais da conta, para uso exclusivo do adapter de PSP.
func (s Contas) Credenciais(ctx context.Context, id string) (domain.Credenciais, error) {
	var cred domain.Credenciais
	cifrado, err := s.Store.Contas().BuscarCredenciais(ctx, id)
	if err != nil {
		return cred, err
	}
	plano, err := s.Vault.Decifrar(ctx, cifrado)
	if err != nil {
		return cred, fmt.Errorf("credenciais da conta %s: %w", id, err)
	}
	if err := json.Unmarshal(plano, &cred); err != nil {
		return cred, fmt.Errorf("credenciais da conta %s corrompidas: %w", id, err)
	}
	return cred, nil
}
