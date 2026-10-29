package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

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

// Trocar o certificado da conta zera o aviso de vencimento, porque o aviso é
// guardado pela impressão digital (RN-62). Ainda não há comando de troca; o teste
// troca direto no banco.
func TestTrocarCertificadoZeraOAvisoDeVencimento(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lastro.db")
	s := abrir(t, path)
	ctx := t.Context()
	c := domain.Conta{ID: "c1", Nome: "Loja", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: domain.Producao, Ativa: true,
		Certificado: &domain.Certificado{Sujeito: "CN=loja", ImpressaoDigital: "velho", Validade: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}}
	if err := s.Contas().Criar(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := s.Contas().MarcarVencimentoAvisado(ctx, "c1", "velho"); err != nil {
		t.Fatal(err)
	}

	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `UPDATE contas SET cert_impressao_digital = 'novo' WHERE id = 'c1'`); err != nil {
		t.Fatal(err)
	}
	got, err := s.Contas().Buscar(ctx, "c1")
	if err != nil || got.Certificado.VencimentoAvisado {
		t.Fatalf("depois de trocar o certificado: avisado=%v, %v; quer false", got.Certificado.VencimentoAvisado, err)
	}
}
