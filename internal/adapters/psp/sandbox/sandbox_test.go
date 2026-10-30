package sandbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
	"github.com/vintemhq/vintem/test/conformance"
)

var (
	inicio = time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	conta  = domain.Conta{ID: "loja", Nome: "Loja Sandbox", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: domain.Homologacao, Ativa: true}
)

func TestConformidade(t *testing.T) {
	conformance.Run(t, func(*testing.T) conformance.Alvo {
		clock := fake.NewClock(inicio)
		return conformance.Alvo{
			PSP:   sandbox.New(clock),
			Conta: conta,
			Agora: clock.Now,
			Pagar: func(*testing.T, domain.Txid) { clock.Avancar(sandbox.AtrasoPadrao) },
		}
	})
}

func criar(t *testing.T, p *sandbox.PSP, clock *fake.Clock, txid string, valor domain.Money) domain.Cobranca {
	t.Helper()
	c, err := p.CriarCob(context.Background(), conta, domain.Cobranca{Txid: domain.Txid(txid), Valor: valor, Expiracao: time.Hour, CriadaEm: clock.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func pixDaJanela(t *testing.T, p *sandbox.PSP) []domain.PixRecebido {
	t.Helper()
	pix, err := p.ListarPix(context.Background(), conta, ports.Janela{Inicio: inicio, Fim: inicio.Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	return pix
}

func TestSoPagaDepoisDoAtraso(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock, sandbox.ComAtraso(10*time.Second))
	c := criar(t, p, clock, "sandbox00000000000000000001", 1050)

	clock.Avancar(9 * time.Second)
	if got, _ := p.ConsultarCob(context.Background(), conta, c.Txid); got.Status != domain.CobrancaAtiva {
		t.Fatalf("pagou antes do atraso: %s", got.Status)
	}
	clock.Avancar(time.Second)
	if got, _ := p.ConsultarCob(context.Background(), conta, c.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("não pagou no atraso: %s", got.Status)
	}
}

func TestValorMagicoRecusado(t *testing.T) {
	clock := fake.NewClock(inicio)
	_, err := sandbox.New(clock).CriarCob(context.Background(), conta, domain.Cobranca{Txid: "sandbox00000000000000000002", Valor: sandbox.ValorRecusado})
	if !errors.Is(err, sandbox.ErrRecusado) {
		t.Fatalf("err = %v", err)
	}
}

func TestValorMagicoNuncaPaga(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock)
	c := criar(t, p, clock, "sandbox00000000000000000003", sandbox.ValorNuncaPaga)
	clock.Avancar(2 * time.Hour)
	got, _ := p.ConsultarCob(context.Background(), conta, c.Txid)
	if got.Status != domain.CobrancaAtiva || !got.Expirada(clock.Now()) || len(pixDaJanela(t, p)) != 0 {
		t.Fatalf("status=%s expirada=%v", got.Status, got.Expirada(clock.Now()))
	}
}

func TestValorMagicoDivergente(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock)
	criar(t, p, clock, "sandbox00000000000000000004", sandbox.ValorDivergente)
	clock.Avancar(sandbox.AtrasoPadrao)
	if pix := pixDaJanela(t, p); len(pix) != 1 || pix[0].Valor != 100 {
		t.Fatalf("pix = %+v", pix)
	}
}

func TestValorMagicoDuplicado(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock)
	criar(t, p, clock, "sandbox00000000000000000005", sandbox.ValorDuplicado)
	clock.Avancar(sandbox.AtrasoPadrao)
	pix := pixDaJanela(t, p)
	if len(pix) != 2 || pix[0].EndToEndID != pix[1].EndToEndID {
		t.Fatalf("esperava o mesmo Pix duas vezes: %+v", pix)
	}
}

func TestNaoPagaDepoisDeExpirar(t *testing.T) {
	clock := fake.NewClock(inicio)
	p := sandbox.New(clock, sandbox.ComAtraso(2*time.Hour))
	c := criar(t, p, clock, "sandbox00000000000000000006", 1050) // expira em 1 h
	clock.Avancar(3 * time.Hour)
	if got, _ := p.ConsultarCob(context.Background(), conta, c.Txid); got.Status != domain.CobrancaAtiva {
		t.Fatalf("pagou cobrança expirada: %s", got.Status)
	}
}
