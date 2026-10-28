package app

import (
	"context"

	"github.com/lastrohq/lastro/internal/domain"
)

type chaveAcesso struct{}

// Acesso é quem está chamando o caso de uso. Vem da API key autenticada.
type Acesso struct {
	APIKeyID string
	// ContaID restringe a chamada a uma conta; vazio vale para todas.
	ContaID string
}

// ComAcesso anexa o acesso ao contexto. Chamadas sem acesso (CLI, jobs
// internos) enxergam todas as contas.
func ComAcesso(ctx context.Context, a Acesso) context.Context {
	return context.WithValue(ctx, chaveAcesso{}, a)
}

// AcessoDe devolve o acesso do contexto, ou o acesso irrestrito.
func AcessoDe(ctx context.Context) Acesso {
	a, _ := ctx.Value(chaveAcesso{}).(Acesso)
	return a
}

// enxerga diz se o acesso pode ver recursos da conta.
func (a Acesso) enxerga(contaID string) bool { return a.ContaID == "" || a.ContaID == contaID }

// AcessoDaChave monta o acesso a partir da API key.
func AcessoDaChave(k domain.APIKey) Acesso { return Acesso{APIKeyID: k.ID, ContaID: k.ContaID} }
