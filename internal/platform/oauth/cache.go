package oauth

import (
	"context"
	"sync"
	"time"
)

// Buscar pede um token novo ao banco. Cada adapter implementa do seu jeito
// (JSON ou form, Basic auth, scope, mTLS).
type Buscar func(ctx context.Context) (Token, error)

// Relogio é a fonte de tempo; ports.Clock e fake.Clock servem.
type Relogio interface {
	Now() time.Time
}

// Cache guarda um token por chave (a conta) e o reutiliza até perto de expirar.
// Chamadas concorrentes para a mesma chave fazem uma única busca.
type Cache struct {
	relogio      Relogio
	antecedencia time.Duration
	timeout      time.Duration

	mu       sync.Mutex
	entradas map[string]*entrada
}

type entrada struct {
	token Token
	voo   *busca // busca em andamento; nil quando não há
}

type busca struct {
	pronto chan struct{}
	token  Token
	err    error
}

// Opcao configura o Cache.
type Opcao func(*Cache)

// ComAntecedencia define quanto antes de expirar o token é renovado (padrão
// 60 s). Em token de vida curta, a antecedência é limitada à metade da vida.
func ComAntecedencia(d time.Duration) Opcao { return func(c *Cache) { c.antecedencia = d } }

// ComTimeout limita cada busca de token (padrão 30 s). A busca não é
// cancelada quando quem a iniciou desiste, porque outras chamadas esperam por ela.
func ComTimeout(d time.Duration) Opcao { return func(c *Cache) { c.timeout = d } }

// NovoCache cria um cache vazio.
func NovoCache(relogio Relogio, opcoes ...Opcao) *Cache {
	c := &Cache{
		relogio:      relogio,
		antecedencia: 60 * time.Second,
		timeout:      30 * time.Second,
		entradas:     map[string]*entrada{},
	}
	for _, o := range opcoes {
		o(c)
	}
	return c
}

// Token devolve o token da chave, buscando um novo se não houver, se estiver
// perto de expirar ou se tiver sido invalidado. Erro de busca não fica guardado:
// a chamada seguinte tenta de novo.
func (c *Cache) Token(ctx context.Context, chave string, buscar Buscar) (Token, error) {
	c.mu.Lock()
	e := c.entradas[chave]
	if e == nil {
		e = &entrada{}
		c.entradas[chave] = e
	}
	if c.valido(e.token) {
		t := e.token
		c.mu.Unlock()
		return t, nil
	}
	b := e.voo
	if b == nil {
		b = &busca{pronto: make(chan struct{})}
		e.voo = b
		go c.buscar(context.WithoutCancel(ctx), e, b, buscar)
	}
	c.mu.Unlock()

	select {
	case <-b.pronto:
		return b.token, b.err
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
}

func (c *Cache) buscar(ctx context.Context, e *entrada, b *busca, buscar Buscar) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	t, err := buscar(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()
	e.voo = nil
	if err == nil {
		e.token = t
	}
	b.token, b.err = t, err
	close(b.pronto)
}

// Invalidar descarta o token da chave se ele ainda for o recusado. Se outra
// chamada já trocou o token, o novo continua valendo.
func (c *Cache) Invalidar(chave string, recusado Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entradas[chave]; e != nil && e.token.Valor == recusado.Valor {
		e.token = Token{}
	}
}

// valido diz se o token ainda serve, já descontada a antecedência.
func (c *Cache) valido(t Token) bool {
	if t.Valor == "" {
		return false
	}
	antecedencia := c.antecedencia
	if metade := t.ExpiraEm.Sub(t.ObtidoEm) / 2; metade < antecedencia {
		antecedencia = metade
	}
	return c.relogio.Now().Before(t.ExpiraEm.Add(-antecedencia))
}
