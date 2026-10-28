package fake_test

import (
	"testing"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
	"github.com/lastrohq/lastro/internal/ports/storetest"
)

func TestStoreContrato(t *testing.T) {
	storetest.Run(t, func(*testing.T) ports.Store { return fake.NewStore() })
}

func TestOutboxSoRecebeEventoDaTransacaoQueDeuCerto(t *testing.T) {
	s := fake.NewStore()
	_ = s.WithTx(t.Context(), func(tx ports.Store) error {
		return tx.Outbox().Adicionar(t.Context(), domain.Evento{ID: "ok", Tipo: domain.EventoCobrancaCriada})
	})
	if got := s.Eventos(); len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("eventos = %+v", got)
	}
}
