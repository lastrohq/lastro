package fake

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

type dados struct {
	contas    map[string]domain.Conta
	creds     map[string][]byte
	cobrancas map[string]domain.Cobranca // chave: contaID + "/" + txid
	ordem     []string                   // chaves de cobrança em ordem de criação
	outbox    []domain.Evento
}

func (d *dados) clonar() *dados {
	return &dados{
		contas:    maps.Clone(d.contas),
		creds:     maps.Clone(d.creds),
		cobrancas: maps.Clone(d.cobrancas),
		ordem:     slices.Clone(d.ordem),
		outbox:    slices.Clone(d.outbox),
	}
}

// Store é um ports.Store em memória. WithTx trabalha numa cópia e só a
// publica se fn não falhar, o que reproduz o rollback de um banco real.
type Store struct {
	mu   *sync.Mutex
	d    *dados
	emTx bool
}

var _ ports.Store = (*Store)(nil)

// NewStore cria um Store vazio.
func NewStore() *Store {
	return &Store{mu: &sync.Mutex{}, d: &dados{contas: map[string]domain.Conta{}, creds: map[string][]byte{}, cobrancas: map[string]domain.Cobranca{}}}
}

// Eventos devolve o que foi publicado no outbox, para asserções.
func (s *Store) Eventos() []domain.Evento {
	s.lock()
	defer s.unlock()
	return slices.Clone(s.d.outbox)
}

func (s *Store) lock() {
	if !s.emTx {
		s.mu.Lock()
	}
}

func (s *Store) unlock() {
	if !s.emTx {
		s.mu.Unlock()
	}
}

// WithTx executa fn sobre uma cópia isolada dos dados.
func (s *Store) WithTx(_ context.Context, fn func(ports.Store) error) error {
	if s.emTx {
		return fn(s)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tx := &Store{mu: s.mu, d: s.d.clonar(), emTx: true}
	if err := fn(tx); err != nil {
		return err
	}
	s.d = tx.d
	return nil
}

// Contas devolve o repositório de contas.
func (s *Store) Contas() ports.ContaRepo { return contas{s} }

// Cobrancas devolve o repositório de cobranças.
func (s *Store) Cobrancas() ports.CobrancaRepo { return cobrancas{s} }

// Outbox devolve o repositório do outbox.
func (s *Store) Outbox() ports.OutboxRepo { return outbox{s} }

type contas struct{ s *Store }

func (r contas) Criar(_ context.Context, c domain.Conta) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.contas[c.ID]; ok {
		return fmt.Errorf("conta %s: %w", c.ID, ports.ErrJaExiste)
	}
	for _, outra := range r.s.d.contas {
		if outra.ChavePix == c.ChavePix {
			return fmt.Errorf("chave %s já pertence à conta %s: %w", c.ChavePix, outra.ID, ports.ErrJaExiste)
		}
	}
	r.s.d.contas[c.ID] = c
	return nil
}

func (r contas) BuscarPorChave(_ context.Context, chave string) (domain.Conta, error) {
	r.s.lock()
	defer r.s.unlock()
	for _, c := range r.s.d.contas {
		if c.ChavePix == chave {
			return c, nil
		}
	}
	return domain.Conta{}, fmt.Errorf("conta da chave %s: %w", chave, ports.ErrNaoEncontrado)
}

func (r contas) Buscar(_ context.Context, id string) (domain.Conta, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.contas[id]
	if !ok {
		return c, fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return c, nil
}

func (r contas) Listar(_ context.Context) ([]domain.Conta, error) {
	r.s.lock()
	defer r.s.unlock()
	out := slices.Collect(maps.Values(r.s.d.contas))
	slices.SortFunc(out, func(a, b domain.Conta) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (r contas) Atualizar(_ context.Context, c domain.Conta) error {
	r.s.lock()
	defer r.s.unlock()
	atual, ok := r.s.d.contas[c.ID]
	if !ok {
		return fmt.Errorf("conta %s: %w", c.ID, ports.ErrNaoEncontrado)
	}
	c.PSP = atual.PSP
	r.s.d.contas[c.ID] = c
	return nil
}

func (r contas) SalvarCredenciais(_ context.Context, id string, cifrado []byte) error {
	r.s.lock()
	defer r.s.unlock()
	if _, ok := r.s.d.contas[id]; !ok {
		return fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	r.s.d.creds[id] = slices.Clone(cifrado)
	return nil
}

func (r contas) BuscarCredenciais(_ context.Context, id string) ([]byte, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.creds[id]
	if !ok {
		return nil, fmt.Errorf("credenciais da conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return slices.Clone(c), nil
}

type cobrancas struct{ s *Store }

func chave(contaID string, txid domain.Txid) string { return contaID + "/" + string(txid) }

func (r cobrancas) Criar(_ context.Context, c domain.Cobranca) error {
	r.s.lock()
	defer r.s.unlock()
	k := chave(c.ContaID, c.Txid)
	for _, outra := range r.s.d.cobrancas {
		if outra.Txid == c.Txid {
			return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrJaExiste)
		}
	}
	c.Revisao = 0
	r.s.d.cobrancas[k] = c
	r.s.d.ordem = append(r.s.d.ordem, k)
	return nil
}

func (r cobrancas) Buscar(_ context.Context, contaID string, txid domain.Txid) (domain.Cobranca, error) {
	r.s.lock()
	defer r.s.unlock()
	c, ok := r.s.d.cobrancas[chave(contaID, txid)]
	if !ok {
		return c, fmt.Errorf("cobrança %s: %w", txid, ports.ErrNaoEncontrado)
	}
	return c, nil
}

func (r cobrancas) BuscarPorTxid(_ context.Context, txid domain.Txid) (domain.Cobranca, error) {
	r.s.lock()
	defer r.s.unlock()
	for _, c := range r.s.d.cobrancas {
		if c.Txid == txid {
			return c, nil
		}
	}
	return domain.Cobranca{}, fmt.Errorf("cobrança %s: %w", txid, ports.ErrNaoEncontrado)
}

func (r cobrancas) Contar(_ context.Context, f ports.FiltroCobrancas) (int, error) {
	r.s.lock()
	defer r.s.unlock()
	n := 0
	for _, k := range r.s.d.ordem {
		if combina(r.s.d.cobrancas[k], f) {
			n++
		}
	}
	return n, nil
}

func (r cobrancas) Atualizar(_ context.Context, c *domain.Cobranca) error {
	r.s.lock()
	defer r.s.unlock()
	k := chave(c.ContaID, c.Txid)
	atual, ok := r.s.d.cobrancas[k]
	if !ok {
		return fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrNaoEncontrado)
	}
	if atual.Revisao != c.Revisao {
		return fmt.Errorf("cobrança %s na revisão %d, gravando sobre %d: %w", c.Txid, atual.Revisao, c.Revisao, ports.ErrConflito)
	}
	c.Revisao++
	r.s.d.cobrancas[k] = *c
	return nil
}

func (r cobrancas) Listar(_ context.Context, f ports.FiltroCobrancas) ([]domain.Cobranca, string, error) {
	r.s.lock()
	defer r.s.unlock()
	inicio := 0
	if f.Cursor != "" {
		n, err := strconv.Atoi(f.Cursor)
		if err != nil || n < 0 {
			return nil, "", fmt.Errorf("cursor inválido %q", f.Cursor)
		}
		inicio = n
	}
	limite := f.Limite
	if limite <= 0 {
		limite = 100
	}
	var out []domain.Cobranca
	pular := f.Pular
	for i := inicio; i < len(r.s.d.ordem); i++ {
		c := r.s.d.cobrancas[r.s.d.ordem[i]]
		if !combina(c, f) {
			continue
		}
		if pular > 0 {
			pular--
			continue
		}
		if len(out) == limite {
			return out, strconv.Itoa(i), nil
		}
		out = append(out, c)
	}
	return out, "", nil
}

func combina(c domain.Cobranca, f ports.FiltroCobrancas) bool {
	if f.ContaID != "" && c.ContaID != f.ContaID {
		return false
	}
	if f.Status != "" && c.Status != f.Status {
		return false
	}
	if f.Documento != "" && (c.Devedor == nil || c.Devedor.Documento != f.Documento) {
		return false
	}
	if !f.Janela.Inicio.IsZero() && c.CriadaEm.Before(f.Janela.Inicio) {
		return false
	}
	if !f.Janela.Fim.IsZero() && !c.CriadaEm.Before(f.Janela.Fim) {
		return false
	}
	return true
}

type outbox struct{ s *Store }

func (r outbox) Adicionar(_ context.Context, e domain.Evento) error {
	r.s.lock()
	defer r.s.unlock()
	r.s.d.outbox = append(r.s.d.outbox, e)
	return nil
}
