package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

func TestAPIKeysCriarAutenticarRevogar(t *testing.T) {
	store := fake.NewStore()
	svc := app.APIKeys{Store: store, Clock: fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))}
	ctx := context.Background()

	k, token, err := svc.Criar(ctx, "loja", []domain.Escopo{domain.EscopoCobRead}, "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Autenticar(ctx, token)
	if err != nil || got.ID != k.ID {
		t.Fatalf("Autenticar = %+v, %v", got, err)
	}
	for _, ruim := range []string{"", "lst_xxx", token[:len(token)-1] + "x", "lst_" + "zzzzzzzzzzzz" + token[16:]} {
		if _, err := svc.Autenticar(ctx, ruim); !errors.Is(err, domain.ErrNaoAutenticado) {
			t.Errorf("token %q autenticou (err = %v)", ruim, err)
		}
	}
	if err := svc.Revogar(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Autenticar(ctx, token); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatal("chave revogada autenticou")
	}
	if _, _, err := svc.Criar(ctx, "presa", []domain.Escopo{domain.EscopoCobRead}, "conta-que-nao-existe"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("conta inexistente: err = %v", err)
	}
}
