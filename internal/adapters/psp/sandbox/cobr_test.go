package sandbox_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// novaCobrSandbox cria a recorrência e a cobrança de valor v no sandbox.
func novaCobrSandbox(t *testing.T, p *sandbox.PSP, v domain.Money) domain.CobrancaRecorrente {
	t.Helper()
	ctx := context.Background()
	r := recSandbox(domain.Retentativa3Em7Dias)
	r.ValorFixo = v
	rec, err := p.CriarRecorrencia(ctx, conta, r)
	if err != nil {
		t.Fatal(err)
	}
	c, err := p.CriarCobr(ctx, conta, domain.CobrancaRecorrente{
		Txid: "cobrsandbox0000000000000001", RecorrenciaID: rec.ID, Valor: v, Vencimento: domain.NovaData(2026, 11, 10),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func status(t *testing.T, p *sandbox.PSP, txid domain.Txid) domain.CobrancaRecorrente {
	t.Helper()
	c, err := p.ConsultarCobr(context.Background(), conta, txid)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCobrNoSandboxEPagaNoVencimento(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock)
	c := novaCobrSandbox(t, p, 9990)
	if c.Status != domain.CobrCriada {
		t.Fatalf("criada = %s", c.Status)
	}
	clock.Avancar(sandbox.AtrasoPadrao)
	if c := status(t, p, c.Txid); c.Status != domain.CobrAtiva || len(c.Tentativas) != 1 || c.Tentativas[0].Status != domain.TentativaAgendada || c.Tentativas[0].Tipo != domain.TipoAgendamento {
		t.Fatalf("depois de um atraso: %+v", c)
	}
	clock.Avancar(sandbox.AtrasoPadrao)
	got := status(t, p, c.Txid)
	if got.Status != domain.CobrConcluida || got.Tentativas[0].Status != domain.TentativaPaga || len(got.Tentativas[0].EndToEndID) != 32 {
		t.Fatalf("depois de dois atrasos: %+v", got)
	}
	if _, err := p.CancelarCobr(context.Background(), conta, c.Txid); !errors.Is(err, domain.ErrTransicaoInvalida) {
		t.Fatalf("cancelar concluída: %v", err)
	}
}

func TestCobrSemSaldoERetentativa(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock)
	c := novaCobrSandbox(t, p, sandbox.ValorCobrSemSaldo)
	clock.Avancar(2 * sandbox.AtrasoPadrao)
	if got := status(t, p, c.Txid); got.Status != domain.CobrAtiva || got.Tentativas[0].Status != domain.TentativaRejeitada || got.Tentativas[0].MotivoRejeicao == "" {
		t.Fatalf("sem saldo: %+v", got)
	}
	nova := domain.NovaData(2026, 11, 12)
	got, err := p.SolicitarRetentativa(context.Background(), conta, c.Txid, nova)
	if err != nil || len(got.Tentativas) != 2 || got.Tentativas[1] != (domain.Tentativa{DataLiquidacao: nova, Tipo: domain.TipoNovaTentativa, Status: domain.TentativaAgendada}) {
		t.Fatalf("retentativa = %+v, %v", got, err)
	}
	clock.Avancar(sandbox.AtrasoPadrao)
	if got := status(t, p, c.Txid); got.Status != domain.CobrConcluida || got.Tentativas[1].Status != domain.TentativaPaga {
		t.Fatalf("retentativa paga: %+v", got)
	}
}

func TestCobrRejeitadaECancelada(t *testing.T) {
	ctx := context.Background()
	t.Run("rejeitada pelo PSP do pagador", func(t *testing.T) {
		clock := fake.NewClock(inicio)
		p := sandbox.New(clock)
		c := novaCobrSandbox(t, p, sandbox.ValorCobrRejeitada)
		clock.Avancar(sandbox.AtrasoPadrao)
		if got := status(t, p, c.Txid); got.Status != domain.CobrRejeitada || got.MotivoEncerramento == "" {
			t.Fatalf("rejeitada: %+v", got)
		}
		if _, err := p.SolicitarRetentativa(ctx, conta, c.Txid, domain.NovaData(2026, 11, 12)); !errors.Is(err, domain.ErrTransicaoInvalida) {
			t.Fatalf("retentativa de rejeitada: %v", err)
		}
	})
	t.Run("cancelada antes de liquidar não é paga", func(t *testing.T) {
		clock := fake.NewClock(inicio)
		p := sandbox.New(clock)
		c := novaCobrSandbox(t, p, 9990)
		clock.Avancar(sandbox.AtrasoPadrao)
		got, err := p.CancelarCobr(ctx, conta, c.Txid)
		if err != nil || got.Status != domain.CobrCancelada || got.Tentativas[0].Status != domain.TentativaCancelada {
			t.Fatalf("cancelada: %+v, %v", got, err)
		}
		clock.Avancar(sandbox.AtrasoPadrao)
		if got := status(t, p, c.Txid); got.Status != domain.CobrCancelada {
			t.Fatalf("depois de cancelada: %s", got.Status)
		}
	})
	t.Run("recusada, de recorrência desconhecida ou de outra conta", func(t *testing.T) {
		p := sandbox.New(fake.NewClock(inicio))
		if _, err := p.CriarCobr(ctx, conta, domain.CobrancaRecorrente{Txid: "cobrsandbox0000000000000002", Valor: sandbox.ValorRecusado}); !errors.Is(err, sandbox.ErrRecusado) {
			t.Fatalf("valor recusado: %v", err)
		}
		if _, err := p.CriarCobr(ctx, conta, domain.CobrancaRecorrente{Txid: "cobrsandbox0000000000000003", RecorrenciaID: "RR0000000020261029naoexiste1", Valor: 100}); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("recorrência desconhecida: %v", err)
		}
		c := novaCobrSandbox(t, p, 9990)
		outra := conta
		outra.ID = "outra"
		if _, err := p.ConsultarCobr(ctx, outra, c.Txid); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("outra conta: %v", err)
		}
	})
}

func TestCobrLembradaDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	ctx := context.Background()
	_ = store.Contas().Criar(ctx, conta)
	antes := sandbox.New(clock, sandbox.ComStore(store))
	r := recSandbox(domain.Retentativa3Em7Dias)
	r.ValorFixo = sandbox.ValorCobrSemSaldo
	rec, _ := antes.CriarRecorrencia(ctx, conta, r)
	rec.Status = domain.RecorrenciaAprovada
	if err := store.Recorrencias().Criar(ctx, rec); err != nil {
		t.Fatal(err)
	}
	c, err := antes.CriarCobr(ctx, conta, domain.CobrancaRecorrente{Txid: "cobrsandbox0000000000000004", RecorrenciaID: rec.ID, Valor: r.ValorFixo, Vencimento: domain.NovaData(2026, 11, 10)})
	if err != nil {
		t.Fatal(err)
	}
	// O Lastro grava a cobrança com a retentativa pedida antes do restart.
	c.Status = domain.CobrAtiva
	c.Tentativas = []domain.Tentativa{
		{DataLiquidacao: c.Vencimento, Tipo: domain.TipoAgendamento, Status: domain.TentativaRejeitada},
		{DataLiquidacao: domain.NovaData(2026, 11, 12), Tipo: domain.TipoNovaTentativa, Status: domain.TentativaAgendada},
	}
	if err := store.CobrancasRecorrentes().Criar(ctx, c); err != nil {
		t.Fatal(err)
	}
	clock.Avancar(3 * sandbox.AtrasoPadrao)
	depois := sandbox.New(clock, sandbox.ComStore(store))
	got, err := depois.ConsultarCobr(ctx, conta, c.Txid)
	if err != nil || got.Status != domain.CobrConcluida || len(got.Tentativas) != 2 || got.Tentativas[1].Status != domain.TentativaPaga {
		t.Fatalf("depois do restart: %+v, %v", got, err)
	}
}

func TestCobrCanceladaContinuaCanceladaDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	ctx := context.Background()
	_ = store.Contas().Criar(ctx, conta)
	antes := sandbox.New(clock, sandbox.ComStore(store))
	rec, _ := antes.CriarRecorrencia(ctx, conta, recSandbox(domain.Retentativa3Em7Dias))
	_ = store.Recorrencias().Criar(ctx, rec)
	c, _ := antes.CriarCobr(ctx, conta, domain.CobrancaRecorrente{Txid: "cobrsandbox0000000000000005", RecorrenciaID: rec.ID, Valor: 100, Vencimento: domain.NovaData(2026, 11, 10)})
	cancelada, err := antes.CancelarCobr(ctx, conta, c.Txid)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CobrancasRecorrentes().Criar(ctx, cancelada); err != nil {
		t.Fatal(err)
	}
	clock.Avancar(3 * sandbox.AtrasoPadrao)
	if got, err := sandbox.New(clock, sandbox.ComStore(store)).ConsultarCobr(ctx, conta, c.Txid); err != nil || got.Status != domain.CobrCancelada {
		t.Fatalf("depois do restart: %+v, %v", got, err)
	}
}
