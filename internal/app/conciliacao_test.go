package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
)

func TestConciliacaoRecuperaPixSemAvisoUmaVezSo(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	normal, _ := c.svc.Criar(ctx, pedido())
	dup := pedido()
	dup.Valor = "2.02" // valor mágico: o sandbox lista o mesmo Pix duas vezes
	duplicado, _ := c.svc.Criar(ctx, dup)

	conc := app.Conciliacao{Store: c.store, PSPs: c.svc.PSPs, Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}, Clock: c.clock}
	if r, _ := conc.Rodar(ctx); r.Novos != 0 {
		t.Fatalf("antes do pagamento recuperou %d", r.Novos)
	}
	// A janela é [agora-2h, agora): o job roda depois do instante do pagamento.
	c.clock.Avancar(sandbox.AtrasoPadrao + time.Second)
	r, err := conc.Rodar(ctx)
	if err != nil || r.Novos != 2 || r.Falhas != 0 || r.Contas != 2 {
		t.Fatalf("rodada: %+v, %v", r, err)
	}
	for _, tx := range []domain.Txid{normal.Txid, duplicado.Txid} {
		if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, tx); got.Status != domain.CobrancaConcluida {
			t.Fatalf("cob %s: %s", tx, got.Status)
		}
	}
	// Rodar de novo não duplica nada.
	if r, _ := conc.Rodar(ctx); r.Novos != 0 {
		t.Fatalf("segunda rodada recuperou %d", r.Novos)
	}
	if n := len(eventosDoTipo(c.store.Eventos(), domain.EventoPixRecebido)); n != 2 {
		t.Fatalf("%d eventos pix.recebido, quer 2", n)
	}
}

func TestConciliacaoRespeitaAJanela(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	_, _ = c.svc.Criar(ctx, pedido())
	c.clock.Avancar(3 * time.Hour) // pago há quase 3 h
	conc := app.Conciliacao{Store: c.store, PSPs: c.svc.PSPs, Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}, Clock: c.clock, Janela: time.Hour}
	if r, _ := conc.Rodar(ctx); r.Novos != 0 {
		t.Fatalf("janela de 1 h recuperou Pix de 3 h atrás: %d", r.Novos)
	}
	conc.Janela = 4 * time.Hour
	if r, _ := conc.Rodar(ctx); r.Novos != 1 {
		t.Fatalf("janela de 4 h: %d", r.Novos)
	}
}

func TestConciliacaoContinuaQuandoUmaContaFalha(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	_ = c.store.Contas().Criar(ctx, domain.Conta{ID: "orfa", Nome: "Órfã", PSP: "banco-removido", ChavePix: "orfa@example.com", Ambiente: domain.Homologacao, Ativa: true})
	if _, err := c.svc.Criar(ctx, pedido()); err != nil {
		t.Fatalf("criar: %v", err)
	}
	c.clock.Avancar(sandbox.AtrasoPadrao + time.Second)
	conc := app.Conciliacao{Store: c.store, PSPs: c.svc.PSPs, Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}, Clock: c.clock}
	r, err := conc.Rodar(ctx)
	if err != nil || r.Falhas != 1 || r.Novos != 1 {
		t.Fatalf("rodada: %+v, %v", r, err)
	}
}
