package psp

import (
	"errors"
	"strings"
	"testing"

	"github.com/lastrohq/lastro/internal/ports"
)

type nomeado struct {
	ports.PSP
	nome string
}

func (n nomeado) Nome() string { return n.nome }

func TestRegistry(t *testing.T) {
	r := NewRegistry(nomeado{nome: "sandbox"}, nomeado{nome: "efi"})
	if p, err := r.PSP("efi"); err != nil || p.Nome() != "efi" {
		t.Fatalf("PSP(efi) = %v, %v", p, err)
	}
	_, err := r.PSP("itau")
	if !errors.Is(err, ports.ErrPSPDesconhecido) || !strings.Contains(err.Error(), "efi sandbox") {
		t.Fatalf("err = %v", err)
	}
}

func TestRegistryRecusaNomeRepetido(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("aceitou PSP repetido")
		}
	}()
	NewRegistry(nomeado{nome: "efi"}, nomeado{nome: "efi"})
}
