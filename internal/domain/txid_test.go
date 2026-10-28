package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseTxidLimitesDeTamanho(t *testing.T) {
	casos := map[int]bool{0: false, 25: false, 26: true, 30: true, 35: true, 36: false}
	for n, valido := range casos {
		_, err := ParseTxid(strings.Repeat("a", n))
		if valido && err != nil {
			t.Errorf("tamanho %d: erro inesperado %v", n, err)
		}
		if !valido && !errors.Is(err, ErrTxidInvalido) {
			t.Errorf("tamanho %d: err = %v, quer ErrTxidInvalido", n, err)
		}
	}
}

func TestParseTxidRecusaCaractereForaDoAlfabeto(t *testing.T) {
	base := strings.Repeat("a", 30)
	for _, c := range []string{"-", "_", " ", ".", "ç", "é", "/", "\x00"} {
		if _, err := ParseTxid(base + c); !errors.Is(err, ErrTxidInvalido) {
			t.Errorf("caractere %q aceito", c)
		}
	}
	if _, err := ParseTxid("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"[:35]); err != nil {
		t.Errorf("maiúsculas e dígitos recusados: %v", err)
	}
}

func TestNewTxidSempreValidoEUnico(t *testing.T) {
	vistos := make(map[Txid]bool)
	for range 10_000 {
		tx := NewTxid()
		if _, err := ParseTxid(string(tx)); err != nil {
			t.Fatalf("NewTxid gerou inválido %q: %v", tx, err)
		}
		if len(tx) != txidGerado {
			t.Fatalf("tamanho %d, quer %d", len(tx), txidGerado)
		}
		if vistos[tx] {
			t.Fatalf("txid repetido: %q", tx)
		}
		vistos[tx] = true
	}
}

func TestNewTxidUsaOAlfabetoInteiro(t *testing.T) {
	contagem := make(map[rune]int)
	for range 2_000 {
		for _, c := range NewTxid() {
			contagem[c]++
		}
	}
	if len(contagem) != len(alfabetoTxid) {
		t.Fatalf("usou %d de %d caracteres do alfabeto", len(contagem), len(alfabetoTxid))
	}
}

func FuzzParseTxid(f *testing.F) {
	f.Add("abcdefghijklmnopqrstuvwxyz")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		tx, err := ParseTxid(s)
		if err != nil {
			return
		}
		if len(tx) < TxidMin || len(tx) > TxidMax {
			t.Fatalf("aceitou tamanho %d", len(tx))
		}
	})
}
