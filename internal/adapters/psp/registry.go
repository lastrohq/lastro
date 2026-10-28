// Package psp reúne os adapters de banco (um subpacote por PSP) e o registry
// que os encontra pelo nome gravado na conta.
package psp

import (
	"fmt"
	"slices"

	"github.com/lastrohq/lastro/internal/ports"
)

// Registry é o ports.Registry montado no main com os adapters disponíveis.
type Registry struct {
	psps map[string]ports.PSP
}

var _ ports.Registry = (*Registry)(nil)

// NewRegistry registra os adapters; nome repetido é erro de programação.
func NewRegistry(psps ...ports.PSP) *Registry {
	r := &Registry{psps: make(map[string]ports.PSP, len(psps))}
	for _, p := range psps {
		if _, dup := r.psps[p.Nome()]; dup {
			panic("psp registrado duas vezes: " + p.Nome())
		}
		r.psps[p.Nome()] = p
	}
	return r
}

// PSP devolve o adapter pelo nome, ou ErrPSPDesconhecido listando os disponíveis.
func (r *Registry) PSP(nome string) (ports.PSP, error) {
	p, ok := r.psps[nome]
	if !ok {
		return nil, fmt.Errorf("%w: %q (disponíveis: %v)", ports.ErrPSPDesconhecido, nome, r.Nomes())
	}
	return p, nil
}

// Nomes lista os adapters registrados, em ordem alfabética.
func (r *Registry) Nomes() []string {
	nomes := make([]string, 0, len(r.psps))
	for n := range r.psps {
		nomes = append(nomes, n)
	}
	slices.Sort(nomes)
	return nomes
}
