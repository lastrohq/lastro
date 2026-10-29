// Package vault cifra segredos em repouso com AES-256-GCM (RN-61).
package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// versao prefixa todo texto cifrado, para permitir trocar o algoritmo ou
// rotacionar a chave sem perder o que já foi gravado.
const versao byte = 1

// ErrChaveInvalida indica uma master key fora do formato.
var ErrChaveInvalida = errors.New("LASTRO_MASTER_KEY inválida: use 32 bytes em base64 (gere com `lastro chave gerar`)")

// ErrDecifrar indica texto cifrado corrompido ou cifrado com outra chave.
var ErrDecifrar = errors.New("não foi possível decifrar: dado corrompido ou master key diferente")

// AESGCM implementa o ports.Vault com uma chave de 256 bits. A checagem fica
// no teste, porque platform não importa o resto do projeto (ver ARQUITETURA.md).
type AESGCM struct {
	aead cipher.AEAD
}

// New cria o vault a partir da master key em base64 (32 bytes decodificados).
func New(chaveBase64 string) (*AESGCM, error) {
	chave, err := base64.StdEncoding.DecodeString(chaveBase64)
	if err != nil || len(chave) != 32 {
		return nil, ErrChaveInvalida
	}
	bloco, err := aes.NewCipher(chave)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(bloco)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &AESGCM{aead: aead}, nil
}

// NovaChave gera uma master key aleatória em base64.
func NovaChave() string {
	chave := make([]byte, 32)
	_, _ = rand.Read(chave) // crypto/rand.Read nunca falha desde o Go 1.24
	return base64.StdEncoding.EncodeToString(chave)
}

// Cifrar devolve versão || nonce || texto cifrado com tag.
func (v *AESGCM) Cifrar(_ context.Context, segredo []byte) ([]byte, error) {
	nonce := make([]byte, v.aead.NonceSize())
	_, _ = rand.Read(nonce)
	out := append([]byte{versao}, nonce...)
	return v.aead.Seal(out, nonce, segredo, []byte{versao}), nil
}

// Decifrar reverte Cifrar; qualquer byte alterado faz falhar com ErrDecifrar.
func (v *AESGCM) Decifrar(_ context.Context, cifrado []byte) ([]byte, error) {
	n := v.aead.NonceSize()
	if len(cifrado) < 1+n+v.aead.Overhead() || cifrado[0] != versao {
		return nil, ErrDecifrar
	}
	plano, err := v.aead.Open(nil, cifrado[1:1+n], cifrado[1+n:], []byte{versao})
	if err != nil {
		return nil, ErrDecifrar
	}
	return plano, nil
}
