package sandbox_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
	"github.com/lastrohq/lastro/test/conformance"
)

// antesDoRestart cria a cobrança no sandbox e a grava no store, como o caso
// de uso faz (o PSP primeiro, o store depois).
func antesDoRestart(t *testing.T, store *fake.Store, clock *fake.Clock, txid string, valor domain.Money) domain.Cobranca {
	t.Helper()
	ctx := context.Background()
	if err := store.Contas().Criar(ctx, conta); err != nil && !isJaExiste(err) {
		t.Fatal(err)
	}
	antigo := sandbox.New(clock, sandbox.ComStore(store))
	c, err := antigo.CriarCob(ctx, conta, domain.Cobranca{Txid: domain.Txid(txid), Valor: valor, Expiracao: time.Hour, CriadaEm: clock.Now(), Chave: conta.ChavePix})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Cobrancas().Criar(ctx, c); err != nil {
		t.Fatal(err)
	}
	return c
}

func isJaExiste(err error) bool { return errors.Is(err, ports.ErrJaExiste) }

func TestCobrancaCriadaAntesDoRestartEPagaDepois(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	c := antesDoRestart(t, store, clock, "restart000000000000000000001", 2500)

	clock.Avancar(sandbox.AtrasoPadrao)
	novo := sandbox.New(clock, sandbox.ComStore(store)) // o processo reiniciou: memória vazia
	ctx := context.Background()

	lida, err := novo.ConsultarCob(ctx, conta, c.Txid)
	if err != nil || lida.Status != domain.CobrancaConcluida {
		t.Fatalf("depois do restart: %+v, %v; quer CONCLUIDA", lida.Status, err)
	}
	pix, err := novo.ListarPix(ctx, conta, ports.Janela{Inicio: inicio, Fim: clock.Now().Add(time.Second)})
	if err != nil || len(pix) != 1 || pix[0].Txid != c.Txid || pix[0].Valor != 2500 {
		t.Fatalf("ListarPix depois do restart = %+v, %v", pix, err)
	}

	// O mesmo Pix que o sandbox antigo teria listado (mesmo endToEndId): a deduplicação do Lastro segura.
	antigo := sandbox.New(clock)
	_, _ = antigo.CriarCob(ctx, conta, domain.Cobranca{Txid: c.Txid, Valor: 2500, Expiracao: time.Hour, CriadaEm: inicio})
	pixAntigo, _ := antigo.ListarPix(ctx, conta, ports.Janela{Inicio: inicio, Fim: clock.Now().Add(time.Second)})
	if len(pixAntigo) != 1 || pixAntigo[0].EndToEndID != pix[0].EndToEndID {
		t.Fatalf("endToEndId mudou com o restart: %v × %v", pixAntigo, pix)
	}
}

func TestValoresMagicosDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	nunca := antesDoRestart(t, store, clock, "restart000000000000000000002", sandbox.ValorNuncaPaga)
	divergente := antesDoRestart(t, store, clock, "restart000000000000000000003", sandbox.ValorDivergente)
	duplicado := antesDoRestart(t, store, clock, "restart000000000000000000004", sandbox.ValorDuplicado)

	clock.Avancar(sandbox.AtrasoPadrao)
	novo := sandbox.New(clock, sandbox.ComStore(store))
	pix, err := novo.ListarPix(context.Background(), conta, ports.Janela{Inicio: inicio, Fim: clock.Now().Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	vezes := map[domain.Txid]int{}
	for _, p := range pix {
		vezes[p.Txid]++
		if p.Txid == divergente.Txid && p.Valor != 100 {
			t.Errorf("0,01 de divergência: pago %s; quer 1,00", p.Valor)
		}
	}
	if vezes[nunca.Txid] != 0 || vezes[divergente.Txid] != 1 || vezes[duplicado.Txid] != 2 {
		t.Fatalf("vezes = %v; quer 0,99 nunca, 1,01 uma vez, 2,02 duas vezes", vezes)
	}
}

func TestEstadoGravadoPeloLastroValeDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	ctx := context.Background()
	paga := antesDoRestart(t, store, clock, "restart000000000000000000005", 2500)
	removida := antesDoRestart(t, store, clock, "restart000000000000000000006", 2500)

	// O Lastro já tinha registrado o pagamento de uma e a remoção da outra.
	for txid, status := range map[domain.Txid]domain.StatusCobranca{paga.Txid: domain.CobrancaConcluida, removida.Txid: domain.CobrancaRemovidaPeloUsuarioRecebedor} {
		c, _ := store.Cobrancas().Buscar(ctx, conta.ID, txid)
		c.Status = status
		if err := store.Cobrancas().Atualizar(ctx, &c); err != nil {
			t.Fatal(err)
		}
	}
	clock.Avancar(sandbox.AtrasoPadrao)
	novo := sandbox.New(clock, sandbox.ComStore(store))
	pix, _ := novo.ListarPix(ctx, conta, ports.Janela{Inicio: inicio, Fim: clock.Now().Add(time.Second)})
	if len(pix) != 1 || pix[0].Txid != paga.Txid {
		t.Fatalf("ListarPix = %+v; quer só a paga (a removida não é paga)", pix)
	}
	if c, _ := novo.ConsultarCob(ctx, conta, removida.Txid); c.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("removida virou %s", c.Status)
	}
	// txid já gravado continua recusado como repetido.
	if _, err := novo.CriarCob(ctx, conta, domain.Cobranca{Txid: paga.Txid, Valor: 100, Expiracao: time.Hour}); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("txid repetido depois do restart: %v", err)
	}
}

func TestCobvLembradaDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	ctx := context.Background()
	_ = store.Contas().Criar(ctx, conta)
	antigo := sandbox.New(clock, sandbox.ComStore(store))
	v := domain.CobrancaVencimento{Txid: "restartv00000000000000000001", Valor: 15000, Vencimento: domain.DataDe(inicio).SomarDias(10), ValidadeAposVencimento: 30, CriadaEm: inicio,
		Devedor: domain.DevedorCompleto{Devedor: domain.Devedor{Documento: "11222333000181", Nome: "Empresa"}}}
	criada, err := antigo.CriarCobv(ctx, conta, v)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CobrancasV().Criar(ctx, criada); err != nil {
		t.Fatal(err)
	}

	clock.Avancar(sandbox.AtrasoPadrao)
	novo := sandbox.New(clock, sandbox.ComStore(store))
	if c, err := novo.ConsultarCobv(ctx, conta, v.Txid); err != nil || c.Status != domain.CobrancaConcluida {
		t.Fatalf("cobv depois do restart: %+v, %v", c.Status, err)
	}
	if pix, _ := novo.ListarPix(ctx, conta, ports.Janela{Inicio: inicio, Fim: clock.Now().Add(time.Second)}); len(pix) != 1 {
		t.Fatalf("Pix da cobv = %+v", pix)
	}
}

func TestConformidadeComStore(t *testing.T) {
	conformance.Run(t, func(*testing.T) conformance.Alvo {
		clock := fake.NewClock(inicio)
		store := fake.NewStore()
		_ = store.Contas().Criar(context.Background(), conta)
		return conformance.Alvo{
			PSP:   sandbox.New(clock, sandbox.ComStore(store)),
			Conta: conta,
			Agora: clock.Now,
			Pagar: func(*testing.T, domain.Txid) { clock.Avancar(sandbox.AtrasoPadrao) },
		}
	})
}
