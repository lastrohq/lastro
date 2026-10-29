package domain

import "time"

// RegistroIdempotencia guarda a resposta de uma requisição com Idempotency-Key (RN-11).
// Status zero significa "em andamento": a primeira requisição ainda não respondeu.
type RegistroIdempotencia struct {
	// Dono é a API key que usou a chave: chaves de clientes diferentes não colidem.
	Dono        string
	Chave       string
	Hash        []byte
	Status      int
	ContentType string
	Corpo       []byte
	CriadaEm    time.Time
}
