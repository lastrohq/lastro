// Package ports define as interfaces que o núcleo exige do mundo externo:
// PSP, Store, Notifier, Vault e Clock. Nada aqui conhece um banco específico.
package ports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
)

// ErrNaoSuportado indica que o PSP da conta não oferece a capacidade pedida (RN-50).
var ErrNaoSuportado = errors.New("capacidade não suportada pelo PSP")

// Capacidade é algo que um PSP pode ou não oferecer.
type Capacidade string

// Capacidades conhecidas (RN-50).
const (
	CapCob       Capacidade = "cob"
	CapCobv      Capacidade = "cobv"
	CapDevolucao Capacidade = "devolucao"
	CapWebhook   Capacidade = "webhook"
	CapRec       Capacidade = "rec"
	CapCobr      Capacidade = "cobr"
)

// Capabilities é o conjunto de capacidades declaradas por um adapter.
type Capabilities map[Capacidade]bool

// Suporta devolve ErrNaoSuportado, com o nome da capacidade, se ela faltar.
func (c Capabilities) Suporta(capacidade Capacidade) error {
	if !c[capacidade] {
		return fmt.Errorf("%w: %s", ErrNaoSuportado, capacidade)
	}
	return nil
}

// Janela é um intervalo de tempo [Inicio, Fim) para consultas como GET /pix.
type Janela struct {
	Inicio, Fim time.Time
}

// PSP é o adapter de um banco ou provedor Pix. Todo desvio do banco em relação
// à spec do BCB é tratado dentro do adapter (RN-51).
type PSP interface {
	// Nome é o identificador do adapter no registry ("efi", "inter", "sandbox"...).
	Nome() string
	Capabilities() Capabilities

	CriarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error)
	ConsultarCob(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.Cobranca, error)
	ListarPix(ctx context.Context, conta domain.Conta, j Janela) ([]domain.PixRecebido, error)
	Devolver(ctx context.Context, conta domain.Conta, d domain.Devolucao) (domain.Devolucao, error)
	ConfigurarWebhook(ctx context.Context, conta domain.Conta, url string) error
}
