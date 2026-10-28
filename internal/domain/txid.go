package domain

import (
	"crypto/rand"
	"errors"
	"fmt"
)

// Txid identifica uma cobrança dentro de uma conta recebedora (RN-10).
type Txid string

// TxidMin é o menor tamanho de txid aceito em cob e cobv.
const TxidMin = 26

// TxidMax é o maior tamanho de txid aceito em cob e cobv.
const TxidMax = 35

// txidGerado é o tamanho do txid gerado pelo Lastro.
const txidGerado = 32

const alfabetoTxid = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// ErrTxidInvalido indica um txid fora do formato da spec do BCB.
var ErrTxidInvalido = errors.New("txid inválido")

// ParseTxid valida um txid informado pelo cliente: de 26 a 35 caracteres em [a-zA-Z0-9].
func ParseTxid(s string) (Txid, error) {
	if len(s) < TxidMin || len(s) > TxidMax {
		return "", fmt.Errorf("%w: tem %d caracteres, deve ter de %d a %d", ErrTxidInvalido, len(s), TxidMin, TxidMax)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return "", fmt.Errorf("%w: caractere %q na posição %d", ErrTxidInvalido, c, i)
		}
	}
	return Txid(s), nil
}

// NewTxid gera um txid aleatório de 32 caracteres com crypto/rand.
func NewTxid() Txid {
	// 248 é o maior múltiplo de 62 abaixo de 256: descartar bytes acima dele
	// evita viés para os primeiros caracteres do alfabeto.
	const limite = 256 - 256%len(alfabetoTxid)
	out := make([]byte, 0, txidGerado)
	buf := make([]byte, txidGerado*2)
	for len(out) < txidGerado {
		_, _ = rand.Read(buf) // crypto/rand.Read nunca falha desde o Go 1.24
		for _, b := range buf {
			if int(b) < limite && len(out) < txidGerado {
				out = append(out, alfabetoTxid[int(b)%len(alfabetoTxid)])
			}
		}
	}
	return Txid(out)
}

func (t Txid) String() string { return string(t) }
