package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/store/sqlite"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/storetest"
)

func abrir(t *testing.T, path string) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStoreContrato(t *testing.T) {
	storetest.Run(t, func(t *testing.T) ports.Store {
		return abrir(t, filepath.Join(t.TempDir(), "lastro.db"))
	})
}

func TestMigrationsAplicamDoZeroEReabrirNaoReaplica(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lastro.db")
	s := abrir(t, path)
	conta := domain.Conta{ID: "c1", Nome: "Loja", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: domain.Producao, Ativa: true}
	if err := s.Contas().Criar(t.Context(), conta); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	got, err := abrir(t, path).Contas().Buscar(t.Context(), "c1")
	if err != nil || got != conta {
		t.Fatalf("depois de reabrir: %+v, %v", got, err)
	}
}

func TestCobrancaDeContaInexistenteFalha(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "lastro.db"))
	err := s.Cobrancas().Criar(t.Context(), domain.Cobranca{Txid: "txid00000000000000000000001", ContaID: "fantasma", Status: domain.CobrancaAtiva})
	if err == nil {
		t.Fatal("aceitou cobrança de conta inexistente")
	}
}

func TestAmbienteInvalidoFalha(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "lastro.db"))
	if err := s.Contas().Criar(t.Context(), domain.Conta{ID: "c1", Ambiente: "teste"}); err == nil {
		t.Fatal("aceitou ambiente fora do CHECK")
	}
}

// Escritas concorrentes não podem dar "database is locked": o busy_timeout e
// as transações IMMEDIATE serializam os escritores.
func TestEscritasConcorrentes(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "lastro.db"))
	ctx := context.Background()
	if err := s.Contas().Criar(ctx, domain.Conta{ID: "c1", Ambiente: domain.Homologacao}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	erros := make(chan error, 50)
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			erros <- s.WithTx(ctx, func(tx ports.Store) error {
				txid := domain.Txid(fmt.Sprintf("txid%023d", i))
				if err := tx.Cobrancas().Criar(ctx, domain.Cobranca{Txid: txid, ContaID: "c1", Status: domain.CobrancaAtiva}); err != nil {
					return err
				}
				return tx.Outbox().Adicionar(ctx, domain.Evento{ID: string(txid), Tipo: domain.EventoCobrancaCriada, ContaID: "c1"})
			})
		}()
	}
	wg.Wait()
	close(erros)
	for err := range erros {
		if err != nil {
			t.Fatal(err)
		}
	}
	pag, _, err := s.Cobrancas().Listar(ctx, ports.FiltroCobrancas{Limite: 100})
	if err != nil || len(pag) != 50 {
		t.Fatalf("listou %d, %v", len(pag), err)
	}
}

func TestEventoComDadosNaoSerializaveisFalha(t *testing.T) {
	s := abrir(t, filepath.Join(t.TempDir(), "lastro.db"))
	err := s.Outbox().Adicionar(t.Context(), domain.Evento{ID: "e1", Dados: make(chan int)})
	if err == nil || errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("err = %v", err)
	}
}
