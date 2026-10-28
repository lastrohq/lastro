package domain

import (
	"errors"
	"fmt"
)

// ErrTransicaoInvalida indica uma mudança de estado fora do desenho da entidade.
var ErrTransicaoInvalida = errors.New("transição de estado inválida")

// Transicoes descreve, para cada estado, para quais estados ele pode ir.
// Estado sem entrada (ou com lista vazia) é terminal.
type Transicoes[S ~string] map[S][]S

// Pode informa se a transição de -> para é permitida.
func (t Transicoes[S]) Pode(de, para S) bool {
	for _, s := range t[de] {
		if s == para {
			return true
		}
	}
	return false
}

// Terminal informa se nenhum estado é alcançável a partir de s.
func (t Transicoes[S]) Terminal(s S) bool { return len(t[s]) == 0 }

// Validar devolve ErrTransicaoInvalida se a transição não for permitida.
func (t Transicoes[S]) Validar(de, para S) error {
	if !t.Pode(de, para) {
		return fmt.Errorf("%w: %s → %s", ErrTransicaoInvalida, de, para)
	}
	return nil
}
