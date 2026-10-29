package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCobrancaExpirada(t *testing.T) {
	criada := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	c := Cobranca{Status: CobrancaAtiva, CriadaEm: criada, Expiracao: time.Hour}

	if c.Expirada(criada.Add(59 * time.Minute)) {
		t.Fatal("expirou antes da hora")
	}
	if !c.Expirada(criada.Add(time.Hour)) {
		t.Fatal("não expirou no limite")
	}
	c.Status = CobrancaConcluida
	if c.Expirada(criada.Add(2 * time.Hour)) {
		t.Fatal("cobrança concluída não expira")
	}
}

func TestCobrancaTransitar(t *testing.T) {
	c := Cobranca{Status: CobrancaAtiva}
	if err := c.Transitar(CobrancaConcluida); err != nil || c.Status != CobrancaConcluida {
		t.Fatalf("err=%v status=%s", err, c.Status)
	}
	if err := c.Transitar(CobrancaAtiva); !errors.Is(err, ErrTransicaoInvalida) || c.Status != CobrancaConcluida {
		t.Fatalf("reativou cobrança concluída: err=%v status=%s", err, c.Status)
	}
}
