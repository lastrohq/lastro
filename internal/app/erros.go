package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrRequisicaoInvalida é o erro-base das violações de entrada.
var ErrRequisicaoInvalida = errors.New("requisição inválida")

// ErrPSP indica que o banco recusou ou falhou. A mensagem original vem embrulhada.
var ErrPSP = errors.New("o PSP recusou ou falhou")

// Violacao aponta o campo (no caminho da spec do BCB, ex.: "valor.original") e o porquê.
type Violacao struct {
	Propriedade string
	Razao       string
	Valor       string
}

// ErroValidacao junta todas as violações de uma requisição, para o cliente
// corrigir tudo de uma vez em vez de descobrir um erro por tentativa.
type ErroValidacao struct {
	Violacoes []Violacao
}

func (e *ErroValidacao) Error() string {
	partes := make([]string, len(e.Violacoes))
	for i, v := range e.Violacoes {
		partes[i] = v.Propriedade + ": " + v.Razao
	}
	return fmt.Sprintf("%s: %s", ErrRequisicaoInvalida, strings.Join(partes, "; "))
}

// Is faz errors.Is(err, ErrRequisicaoInvalida) funcionar.
func (e *ErroValidacao) Is(alvo error) bool { return alvo == ErrRequisicaoInvalida }

type validacao struct{ v []Violacao }

func (b *validacao) add(prop, razao, valor string) {
	b.v = append(b.v, Violacao{Propriedade: prop, Razao: razao, Valor: valor})
}

func (b *validacao) err() error {
	if len(b.v) == 0 {
		return nil
	}
	return &ErroValidacao{Violacoes: b.v}
}

func novoID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
