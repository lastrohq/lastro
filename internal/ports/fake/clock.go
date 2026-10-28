// Package fake traz implementações em memória dos ports, para testes.
package fake

import (
	"sync"
	"time"
)

// Clock é um relógio parado que só anda quando o teste manda.
type Clock struct {
	mu sync.Mutex
	t  time.Time
}

// NewClock cria um relógio parado em t.
func NewClock(t time.Time) *Clock { return &Clock{t: t} }

// Now devolve o instante atual do relógio.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// Avancar move o relógio para frente.
func (c *Clock) Avancar(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}
