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

const idRecCobr = "RR1234567820261029abcDEF00100"

func cobr(txid string, venc domain.Data, minuto int) domain.CobrancaRecorrente {
	return domain.CobrancaRecorrente{
		Txid: domain.Txid(txid), RecorrenciaID: idRecCobr, ContaID: "c1", Status: domain.CobrCriada, Valor: 9990,
		Vencimento: venc, AjusteDiaUtil: true, InfoAdicional: "Mensalidade de novembro",
		CriadaEm: base.Add(time.Duration(minuto) * time.Minute),
	}
}

// comRecorrencia cria um Store com as contas e a recorrência das cobranças recorrentes.
func comRecorrencia(t *testing.T, novo func(t *testing.T) ports.Store) ports.Store {
	t.Helper()
	s := comContas(t, novo)
	r := recorrencia(idRecCobr, "c1")
	r.Status = domain.RecorrenciaAprovada
	if err := s.Recorrencias().Criar(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	return s
}

// runCobr é a parte da suíte para a cobrança recorrente (RN-42).
func runCobr(t *testing.T, novo func(t *testing.T) ports.Store) {
	t.Run("cobr: cria, busca, recusa duplicada e recorrência inexistente", func(t *testing.T) {
		s, ctx := comRecorrencia(t, novo), context.Background()
		c := cobr("cobr0000000000000000000000001", domain.NovaData(2026, 11, 10), 0)
		semNada := cobr("cobr0000000000000000000000002", domain.NovaData(2026, 12, 10), 1)
		semNada.AjusteDiaUtil, semNada.InfoAdicional = false, ""
		for _, x := range []domain.CobrancaRecorrente{c, semNada} {
			if err := s.CobrancasRecorrentes().Criar(ctx, x); err != nil {
				t.Fatal(err)
			}
			got, err := s.CobrancasRecorrentes().BuscarPorTxid(ctx, x.Txid)
			if err != nil || !reflect.DeepEqual(got, x) {
				t.Fatalf("BuscarPorTxid = %+v, %v\nquer          %+v", got, err, x)
			}
		}
		if err := s.CobrancasRecorrentes().Criar(ctx, c); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("duplicada: %v", err)
		}
		orfa := cobr("cobr0000000000000000000000003", domain.NovaData(2026, 11, 10), 2)
		orfa.RecorrenciaID = "RR1234567820261029abcDEF99999"
		if err := s.CobrancasRecorrentes().Criar(ctx, orfa); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("recorrência inexistente: %v", err)
		}
		if _, err := s.CobrancasRecorrentes().BuscarPorTxid(ctx, "cobr0000000000000000000099999"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("cobr: o txid é único entre cob, cobv e cobr", func(t *testing.T) {
		s, ctx := comRecorrencia(t, novo), context.Background()
		_ = s.Cobrancas().Criar(ctx, cob("c1", "txidcomum0000000000000000001", 0))
		_ = s.CobrancasV().Criar(ctx, cobv("c1", "txidcomum0000000000000000002", 0))
		for _, txid := range []string{"txidcomum0000000000000000001", "txidcomum0000000000000000002"} {
			if err := s.CobrancasRecorrentes().Criar(ctx, cobr(txid, domain.NovaData(2026, 11, 10), 0)); !errors.Is(err, ports.ErrJaExiste) {
				t.Fatalf("cobr com txid de outra cobrança (%s): %v", txid, err)
			}
		}
		_ = s.CobrancasRecorrentes().Criar(ctx, cobr("txidcomum0000000000000000003", domain.NovaData(2026, 11, 10), 0))
		if err := s.Cobrancas().Criar(ctx, cob("c1", "txidcomum0000000000000000003", 1)); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("cob com txid de cobr: %v", err)
		}
		if err := s.CobrancasV().Criar(ctx, cobv("c1", "txidcomum0000000000000000003", 1)); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("cobv com txid de cobr: %v", err)
		}
	})

	t.Run("cobr: atualizar grava status, tentativas e motivo, e segue a revisão", func(t *testing.T) {
		s, ctx := comRecorrencia(t, novo), context.Background()
		c := cobr("cobr0000000000000000000000004", domain.NovaData(2026, 11, 10), 0)
		_ = s.CobrancasRecorrentes().Criar(ctx, c)
		velha := c
		c.Status, c.MotivoEncerramento = domain.CobrAtiva, ""
		c.Tentativas = []domain.Tentativa{
			{DataLiquidacao: domain.NovaData(2026, 11, 10), Tipo: domain.TipoAgendamento, Status: domain.TentativaRejeitada,
				EndToEndID: "E12345678202611101200abcdef12345", MotivoRejeicao: "AM04 saldo insuficiente"},
			{DataLiquidacao: domain.NovaData(2026, 11, 12), Tipo: domain.TipoNovaTentativa, Status: domain.TentativaSolicitada},
		}
		if err := s.CobrancasRecorrentes().Atualizar(ctx, &c); err != nil || c.Revisao != 1 {
			t.Fatalf("Atualizar: revisão %d, %v", c.Revisao, err)
		}
		got, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, c.Txid)
		if !reflect.DeepEqual(got, c) {
			t.Fatalf("depois de atualizar = %+v\nquer                 %+v", got, c)
		}
		c.Tentativas[0].Status = domain.TentativaPaga // mexer no slice de quem chamou não muda o gravado
		if got, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, c.Txid); got.Tentativas[0].Status != domain.TentativaRejeitada {
			t.Fatal("o store guardou o slice de quem chamou")
		}
		if err := s.CobrancasRecorrentes().Atualizar(ctx, &velha); !errors.Is(err, ports.ErrConflito) {
			t.Fatalf("revisão velha: %v", err)
		}
		fantasma := cobr("cobr0000000000000000000099999", domain.NovaData(2026, 11, 10), 0)
		if err := s.CobrancasRecorrentes().Atualizar(ctx, &fantasma); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("cobr: as da recorrência vêm em ordem de vencimento", func(t *testing.T) {
		s, ctx := comRecorrencia(t, novo), context.Background()
		outra := recorrencia("RR1234567820261029abcDEF00101", "c1")
		_ = s.Recorrencias().Criar(ctx, outra)
		_ = s.CobrancasRecorrentes().Criar(ctx, cobr("cobr0000000000000000000000012", domain.NovaData(2026, 12, 10), 0))
		_ = s.CobrancasRecorrentes().Criar(ctx, cobr("cobr0000000000000000000000011", domain.NovaData(2026, 11, 10), 1))
		daOutra := cobr("cobr0000000000000000000000013", domain.NovaData(2026, 11, 10), 2)
		daOutra.RecorrenciaID = outra.ID
		_ = s.CobrancasRecorrentes().Criar(ctx, daOutra)
		lista, err := s.CobrancasRecorrentes().DaRecorrencia(ctx, idRecCobr)
		if err != nil || len(lista) != 2 || lista[0].Txid != "cobr0000000000000000000000011" || lista[1].Txid != "cobr0000000000000000000000012" {
			t.Fatalf("DaRecorrencia = %+v, %v", lista, err)
		}
		if vazia, err := s.CobrancasRecorrentes().DaRecorrencia(ctx, "RR1234567820261029abcDEF99999"); err != nil || len(vazia) != 0 {
			t.Fatalf("de recorrência sem cobrança: %+v, %v", vazia, err)
		}
	})
}
