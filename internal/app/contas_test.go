package app_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/vault"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

type pspFalso struct{ ports.PSP }

func (pspFalso) Nome() string { return "sandbox" }

func novoContas(t *testing.T) (app.Contas, *fake.Store) {
	t.Helper()
	v, err := vault.New(vault.NovaChave())
	if err != nil {
		t.Fatal(err)
	}
	s := fake.NewStore()
	return app.Contas{Store: s, Vault: v, PSPs: psp.NewRegistry(pspFalso{})}, s
}

var cred = domain.Credenciais{ClientID: "id-123", ClientSecret: "segredo-xyz", Certificado: []byte("PEM...")}

func conta() domain.Conta {
	return domain.Conta{ID: "loja", Nome: "Loja", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: domain.Homologacao}
}

func TestCriarContaGravaAtivaComCredenciaisCifradas(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	if err := svc.Criar(ctx, conta(), cred); err != nil {
		t.Fatal(err)
	}
	c, err := s.Contas().Buscar(ctx, "loja")
	if err != nil || !c.Ativa {
		t.Fatalf("conta = %+v, %v", c, err)
	}
	blob, _ := s.Contas().BuscarCredenciais(ctx, "loja")
	if bytes.Contains(blob, []byte("segredo-xyz")) || bytes.Contains(blob, []byte("id-123")) {
		t.Fatal("credencial gravada em claro")
	}
	got, err := svc.Credenciais(ctx, "loja")
	if err != nil || got.ClientSecret != cred.ClientSecret || string(got.Certificado) != "PEM..." {
		t.Fatalf("Credenciais = %v, %v", got.ClientID, err)
	}
}

func TestCriarContaRecusa(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()

	invalida := conta()
	invalida.ID = "Com Espaço"
	if err := svc.Criar(ctx, invalida, cred); !errors.Is(err, domain.ErrContaInvalida) {
		t.Fatalf("conta inválida: err = %v", err)
	}
	outroPSP := conta()
	outroPSP.PSP = "banco-inventado"
	if err := svc.Criar(ctx, outroPSP, cred); !errors.Is(err, ports.ErrPSPDesconhecido) {
		t.Fatalf("psp desconhecido: err = %v", err)
	}
	if lista, _ := s.Contas().Listar(ctx); len(lista) != 0 {
		t.Fatalf("gravou conta recusada: %+v", lista)
	}
	_ = svc.Criar(ctx, conta(), cred)
	if err := svc.Criar(ctx, conta(), cred); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("duplicada: err = %v", err)
	}
}

func TestDesativarContaImpedeCobranca(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	_ = svc.Criar(ctx, conta(), cred)
	if err := svc.Desativar(ctx, "loja"); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Contas().Buscar(ctx, "loja")
	if err := c.PodeCobrar(); !errors.Is(err, domain.ErrContaInativa) {
		t.Fatalf("conta desativada ainda cobra: %v", err)
	}
	if err := svc.Desativar(ctx, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("inexistente: err = %v", err)
	}
}

func TestCredenciaisComOutraMasterKeyFalham(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	_ = svc.Criar(ctx, conta(), cred)
	outra, _ := vault.New(vault.NovaChave())
	svc2 := app.Contas{Store: s, Vault: outra, PSPs: svc.PSPs}
	if _, err := svc2.Credenciais(ctx, "loja"); !errors.Is(err, vault.ErrDecifrar) {
		t.Fatalf("err = %v", err)
	}
}
