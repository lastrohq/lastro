package storetest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

const (
	e2eDevolucao  = "E12345678202610291200devolucao01"
	e2eDevolucao2 = "E12345678202610291200devolucao02"
)

func devolucao(e2e, id string, minuto int) domain.Devolucao {
	return domain.Devolucao{
		ID: id, EndToEndID: e2e, ContaID: "c1", Valor: 1000, Natureza: domain.DevolucaoOriginal,
		Descricao: "Produto devolvido", Status: domain.DevolucaoEmProcessamento,
		CriadaEm: base.Add(time.Duration(minuto) * time.Minute),
	}
}

// comPix cria um Store com as contas e dois Pix recebidos da conta c1.
func comPix(t *testing.T, novo func(t *testing.T) ports.Store) ports.Store {
	t.Helper()
	s := comContas(t, novo)
	for _, e2e := range []string{e2eDevolucao, e2eDevolucao2} {
		p := domain.PixRecebido{EndToEndID: e2e, ContaID: "c1", Txid: "txid00000000000000000000001", Valor: 5000, Chave: "c1@example.com", Horario: base}
		if _, err := s.Pix().Registrar(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// runDevolucoes é a parte da suíte para as devoluções (RN-20, RN-21).
func runDevolucoes(t *testing.T, novo func(t *testing.T) ports.Store) {
	t.Run("devolução: cria, busca, recusa par repetido e Pix inexistente", func(t *testing.T) {
		s, ctx := comPix(t, novo), context.Background()
		d := devolucao(e2eDevolucao, "dev1", 0)
		if err := s.Devolucoes().Criar(ctx, d); err != nil {
			t.Fatal(err)
		}
		got, err := s.Devolucoes().Buscar(ctx, d.EndToEndID, d.ID)
		if err != nil || !reflect.DeepEqual(got, d) {
			t.Fatalf("Buscar = %+v, %v\nquer   %+v", got, err, d)
		}
		if err := s.Devolucoes().Criar(ctx, d); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("par repetido: %v", err)
		}
		// O mesmo id em outro Pix é outra devolução.
		if err := s.Devolucoes().Criar(ctx, devolucao(e2eDevolucao2, "dev1", 1)); err != nil {
			t.Fatalf("mesmo id em outro Pix: %v", err)
		}
		if err := s.Devolucoes().Criar(ctx, devolucao("E12345678202610291200inexistente", "dev1", 2)); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("Pix inexistente: %v", err)
		}
		if _, err := s.Devolucoes().Buscar(ctx, e2eDevolucao, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("devolução: atualizar grava o resultado e segue a revisão", func(t *testing.T) {
		s, ctx := comPix(t, novo), context.Background()
		d := devolucao(e2eDevolucao, "dev1", 0)
		if err := s.Devolucoes().Criar(ctx, d); err != nil {
			t.Fatal(err)
		}
		velha := d
		d.Status, d.RtrID, d.LiquidadaEm = domain.DevolucaoDevolvido, "D12345678202610291200abcdefghijk", base.Add(time.Hour)
		if err := s.Devolucoes().Atualizar(ctx, &d); err != nil || d.Revisao != 1 {
			t.Fatalf("Atualizar: revisão %d, %v", d.Revisao, err)
		}
		got, err := s.Devolucoes().Buscar(ctx, d.EndToEndID, d.ID)
		if err != nil || !reflect.DeepEqual(got, d) {
			t.Fatalf("depois de atualizar = %+v, %v\nquer                %+v", got, err, d)
		}
		velha.Status, velha.Motivo = domain.DevolucaoNaoRealizado, "saldo"
		if err := s.Devolucoes().Atualizar(ctx, &velha); !errors.Is(err, ports.ErrConflito) {
			t.Fatalf("revisão velha: %v", err)
		}
		fantasma := devolucao(e2eDevolucao, "nada", 0)
		if err := s.Devolucoes().Atualizar(ctx, &fantasma); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("devolução: do Pix e em processamento vêm em ordem de criação", func(t *testing.T) {
		s, ctx := comPix(t, novo), context.Background()
		tarde, cedo, outro := devolucao(e2eDevolucao, "b", 5), devolucao(e2eDevolucao, "a", 1), devolucao(e2eDevolucao2, "c", 3)
		feita := devolucao(e2eDevolucao, "d", 2)
		feita.Status = domain.DevolucaoDevolvido
		for _, d := range []domain.Devolucao{tarde, cedo, outro, feita} {
			if err := s.Devolucoes().Criar(ctx, d); err != nil {
				t.Fatal(err)
			}
		}
		ids := func(ds []domain.Devolucao) (out []string) {
			for _, d := range ds {
				out = append(out, d.ID)
			}
			return out
		}
		doPix, err := s.Devolucoes().DoPix(ctx, e2eDevolucao)
		if err != nil || !reflect.DeepEqual(ids(doPix), []string{"a", "d", "b"}) {
			t.Fatalf("DoPix = %v, %v", ids(doPix), err)
		}
		abertas, err := s.Devolucoes().EmProcessamento(ctx, "c1")
		if err != nil || !reflect.DeepEqual(ids(abertas), []string{"a", "c", "b"}) {
			t.Fatalf("EmProcessamento = %v, %v", ids(abertas), err)
		}
		if outra, err := s.Devolucoes().EmProcessamento(ctx, "c2"); err != nil || len(outra) != 0 {
			t.Fatalf("conta c2: %v, %v", ids(outra), err)
		}
	})
}
