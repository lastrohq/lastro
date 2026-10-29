package ports

import (
	"context"
	"errors"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
)

// Erros comuns a todo Store.
var (
	ErrNaoEncontrado = errors.New("não encontrado")
	ErrJaExiste      = errors.New("já existe")
	// ErrConflito indica que o registro mudou desde a leitura (revisão desatualizada).
	ErrConflito = errors.New("conflito de revisão")
)

// Store dá acesso aos repositórios e à transação.
type Store interface {
	Contas() ContaRepo
	Cobrancas() CobrancaRepo
	Outbox() OutboxRepo
	APIKeys() APIKeyRepo
	Idempotencia() IdempotenciaRepo
	// WithTx executa fn numa transação: erro em fn desfaz tudo.
	WithTx(ctx context.Context, fn func(Store) error) error
}

// ContaRepo guarda as contas recebedoras e as credenciais delas, já cifradas.
type ContaRepo interface {
	Criar(ctx context.Context, c domain.Conta) error
	Buscar(ctx context.Context, id string) (domain.Conta, error)
	// BuscarPorChave encontra a conta dona da chave Pix (chave é única na instância).
	BuscarPorChave(ctx context.Context, chave string) (domain.Conta, error)
	Listar(ctx context.Context) ([]domain.Conta, error)
	// Atualizar grava nome, chave, ambiente e ativa. ID e PSP não mudam.
	Atualizar(ctx context.Context, c domain.Conta) error
	// SalvarCredenciais grava o blob cifrado pelo Vault; o Store nunca vê o segredo em claro.
	SalvarCredenciais(ctx context.Context, contaID string, cifrado []byte) error
	// BuscarCredenciais devolve o blob cifrado, ou ErrNaoEncontrado se a conta não tiver credenciais.
	BuscarCredenciais(ctx context.Context, contaID string) ([]byte, error)
}

// FiltroCobrancas restringe a listagem; zero vale "sem filtro".
type FiltroCobrancas struct {
	ContaID string
	Status  domain.StatusCobranca
	// Documento filtra pelo CPF/CNPJ do devedor.
	Documento string
	Janela    Janela
	Limite    int
	// Pular descarta os primeiros N resultados (paginação por página da spec do BCB).
	// Com Cursor, prefira deixar zero.
	Pular int
	// Cursor é o valor devolvido pela página anterior; vazio começa do início.
	Cursor string
}

// CobrancaRepo guarda cobranças imediatas. O txid é único na instância (RN-10).
type CobrancaRepo interface {
	// Criar falha com ErrJaExiste se o txid já existir.
	Criar(ctx context.Context, c domain.Cobranca) error
	Buscar(ctx context.Context, contaID string, txid domain.Txid) (domain.Cobranca, error)
	// BuscarPorTxid encontra a cobrança sem saber a conta.
	BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.Cobranca, error)
	// Contar devolve quantas cobranças casam com o filtro, ignorando Limite, Pular e Cursor.
	Contar(ctx context.Context, f FiltroCobrancas) (int, error)
	// Atualizar grava se c.Revisao for a revisão atual, e a incrementa;
	// senão falha com ErrConflito.
	Atualizar(ctx context.Context, c *domain.Cobranca) error
	// Listar devolve uma página e o cursor da próxima (vazio se acabou).
	Listar(ctx context.Context, f FiltroCobrancas) ([]domain.Cobranca, string, error)
}

// OutboxRepo guarda eventos a entregar, na mesma transação da mudança de estado (ADR-005).
type OutboxRepo interface {
	Adicionar(ctx context.Context, e domain.Evento) error
}

// APIKeyRepo guarda as API keys. Só o hash do segredo é gravado (RN-60).
type APIKeyRepo interface {
	Criar(ctx context.Context, k domain.APIKey) error
	Buscar(ctx context.Context, id string) (domain.APIKey, error)
	Listar(ctx context.Context) ([]domain.APIKey, error)
	// Revogar marca a chave como revogada em quando; revogar de novo não muda a data.
	Revogar(ctx context.Context, id string, quando time.Time) error
}

// IdempotenciaRepo guarda as respostas por (dono, chave) para o Idempotency-Key (RN-11).
type IdempotenciaRepo interface {
	// Buscar devolve o registro, ou ErrNaoEncontrado.
	Buscar(ctx context.Context, dono, chave string) (domain.RegistroIdempotencia, error)
	// Salvar grava ou substitui o registro de (dono, chave).
	Salvar(ctx context.Context, r domain.RegistroIdempotencia) error
	// Remover apaga o registro; apagar o que não existe não é erro.
	Remover(ctx context.Context, dono, chave string) error
}
