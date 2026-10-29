package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

func novaIdem() (app.Idempotencia, *fake.Clock) {
	clock := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	return app.Idempotencia{Store: fake.NewStore(), Clock: clock}, clock
}

var (
	hashA = []byte("pedido A")
	hashB = []byte("pedido B")
	resp  = app.RespostaGuardada{Status: 201, ContentType: "application/json", Corpo: []byte(`{"txid":"x"}`)}
)

func TestIdempotenciaRepeteARespostaOriginal(t *testing.T) {
	s, _ := novaIdem()
	ctx := context.Background()
	if g, err := s.Iniciar(ctx, "k1", "c1", hashA); g != nil || err != nil {
		t.Fatalf("primeira vez: %v, %v", g, err)
	}
	if err := s.Concluir(ctx, "k1", "c1", hashA, resp); err != nil {
		t.Fatal(err)
	}
	g, err := s.Iniciar(ctx, "k1", "c1", hashA)
	if err != nil || g == nil || g.Status != 201 || string(g.Corpo) != `{"txid":"x"}` {
		t.Fatalf("repetição: %+v, %v", g, err)
	}
	if _, err := s.Iniciar(ctx, "k1", "c1", hashB); !errors.Is(err, app.ErrIdempotenciaDivergente) {
		t.Fatalf("payload diferente: err = %v", err)
	}
	if g, err := s.Iniciar(ctx, "k2", "c1", hashB); g != nil || err != nil {
		t.Fatalf("outro dono deveria ter a chave livre: %v, %v", g, err)
	}
}

func TestIdempotenciaRecusaConcorrenteEDepoisLiberaReservaAbandonada(t *testing.T) {
	s, clock := novaIdem()
	ctx := context.Background()
	_, _ = s.Iniciar(ctx, "k1", "c1", hashA)
	if _, err := s.Iniciar(ctx, "k1", "c1", hashA); !errors.Is(err, app.ErrIdempotenciaEmAndamento) {
		t.Fatalf("concorrente: err = %v", err)
	}
	clock.Avancar(app.ReservaAbandonada)
	if g, err := s.Iniciar(ctx, "k1", "c1", hashA); g != nil || err != nil {
		t.Fatalf("reserva abandonada deveria liberar: %v, %v", g, err)
	}
}

func TestIdempotenciaNaoGuarda5xx(t *testing.T) {
	s, _ := novaIdem()
	ctx := context.Background()
	_, _ = s.Iniciar(ctx, "k1", "c1", hashA)
	_ = s.Concluir(ctx, "k1", "c1", hashA, app.RespostaGuardada{Status: 502})
	if g, err := s.Iniciar(ctx, "k1", "c1", hashA); g != nil || err != nil {
		t.Fatalf("depois de 5xx a chave deveria estar livre: %v, %v", g, err)
	}
}

func TestIdempotenciaExpiraEm24h(t *testing.T) {
	s, clock := novaIdem()
	ctx := context.Background()
	_, _ = s.Iniciar(ctx, "k1", "c1", hashA)
	_ = s.Concluir(ctx, "k1", "c1", hashA, resp)
	clock.Avancar(app.ValidadeIdempotencia)
	if g, err := s.Iniciar(ctx, "k1", "c1", hashB); g != nil || err != nil {
		t.Fatalf("depois de 24h a chave deveria aceitar outro pedido: %v, %v", g, err)
	}
}
