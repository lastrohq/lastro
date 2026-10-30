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

// Alcanca informa se para é alcançável a partir de de por uma ou mais
// transições. É o que vale para o status que vem do banco: um aviso perdido
// não pode travar a entidade num estado intermediário.
func (t Transicoes[S]) Alcanca(de, para S) bool {
	vistos := map[S]bool{de: true}
	fila := []S{de}
	for len(fila) > 0 {
		atual := fila[0]
		fila = fila[1:]
		for _, prox := range t[atual] {
			if prox == para {
				return true
			}
			if !vistos[prox] {
				vistos[prox] = true
				fila = append(fila, prox)
			}
		}
	}
	return false
}

// ValidarCaminho devolve ErrTransicaoInvalida se para não for alcançável a partir de de.
func (t Transicoes[S]) ValidarCaminho(de, para S) error {
	if !t.Alcanca(de, para) {
		return fmt.Errorf("%w: %s → %s", ErrTransicaoInvalida, de, para)
	}
	return nil
}

// Validar devolve ErrTransicaoInvalida se a transição não for permitida.
func (t Transicoes[S]) Validar(de, para S) error {
	if !t.Pode(de, para) {
		return fmt.Errorf("%w: %s → %s", ErrTransicaoInvalida, de, para)
	}
	return nil
}
