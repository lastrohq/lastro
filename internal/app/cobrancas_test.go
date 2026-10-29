package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
	"github.com/lastrohq/lastro/pkg/brcode"
)

type cenario struct {
	svc   app.Cobrancas
	store *fake.Store
	clock *fake.Clock
}

func novoCenario(t *testing.T) cenario {
	t.Helper()
	clock := fake.NewClock(time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC))
	store := fake.NewStore()
	for _, c := range []domain.Conta{
		{ID: "loja", Nome: "Loja", PSP: "sandbox", ChavePix: "loja@example.com", Ambiente: domain.Homologacao, Ativa: true},
		{ID: "fechada", Nome: "Fechada", PSP: "sandbox", ChavePix: "fechada@example.com", Ambiente: domain.Homologacao, Ativa: false},
	} {
		if err := store.Contas().Criar(context.Background(), c); err != nil {
			t.Fatal(err)
		}
	}
	return cenario{svc: app.Cobrancas{Store: store, PSPs: psp.NewRegistry(sandbox.New(clock)), Clock: clock}, store: store, clock: clock}
}

func pedido() app.PedidoCob {
	return app.PedidoCob{Chave: "loja@example.com", Valor: "10.50", Devedor: &app.PedidoDevedor{CPF: "12345678909", Nome: "Fulano"}}
}

func tipos(evs []domain.Evento) []domain.TipoEvento {
	out := make([]domain.TipoEvento, len(evs))
	for i, e := range evs {
		out[i] = e.Tipo
	}
	return out
}

func TestFluxoCompletoContraOSandbox(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()

	cob, err := c.svc.Criar(ctx, pedido())
	if err != nil {
		t.Fatal(err)
	}
	if cob.Status != domain.CobrancaAtiva || cob.ContaID != "loja" || cob.Valor != 1050 || cob.Revisao != 0 || cob.Expiracao != domain.ExpiracaoPadrao {
		t.Fatalf("criada: %+v", cob)
	}
	if p, err := brcode.Decode(cob.BRCode); err != nil || p.URL != cob.Location || p.Valor != 1050 {
		t.Fatalf("BR Code: %+v, %v", p, err)
	}

	if got, _ := c.svc.Consultar(ctx, string(cob.Txid)); got.Status != domain.CobrancaAtiva {
		t.Fatalf("pago antes da hora: %s", got.Status)
	}
	c.clock.Avancar(sandbox.AtrasoPadrao)
	got, err := c.svc.Consultar(ctx, string(cob.Txid))
	if err != nil || got.Status != domain.CobrancaConcluida || got.Revisao != 1 {
		t.Fatalf("depois do pagamento: %+v, %v", got, err)
	}
	// Consultar de novo não republica o evento.
	_, _ = c.svc.Consultar(ctx, string(cob.Txid))
	quer := []domain.TipoEvento{domain.EventoCobrancaCriada, domain.EventoCobrancaConcluida}
	if evs := tipos(c.store.Eventos()); len(evs) != 2 || evs[0] != quer[0] || evs[1] != quer[1] {
		t.Fatalf("eventos = %v, quer %v", evs, quer)
	}
}

func TestCriarComTxidDoCliente(t *testing.T) {
	c := novoCenario(t)
	p := pedido()
	p.Txid = "pedido0000000000000000000001"
	cob, err := c.svc.Criar(context.Background(), p)
	if err != nil || cob.Txid != "pedido0000000000000000000001" {
		t.Fatalf("%+v, %v", cob, err)
	}
	if _, err := c.svc.Criar(context.Background(), p); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("txid repetido: err = %v", err)
	}
}

func TestCriarJuntaTodasAsViolacoes(t *testing.T) {
	c := novoCenario(t)
	exp := 0
	_, err := c.svc.Criar(context.Background(), app.PedidoCob{
		Txid: "curto", Chave: "", Valor: "10,50", ExpiracaoSegundos: &exp,
		Devedor: &app.PedidoDevedor{CPF: "12345678900", Nome: ""},
	})
	var ev *app.ErroValidacao
	if !errors.As(err, &ev) || !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("err = %v", err)
	}
	props := map[string]bool{}
	for _, v := range ev.Violacoes {
		props[v.Propriedade] = true
	}
	for _, p := range []string{"txid", "chave", "valor.original", "calendario.expiracao", "devedor.cpf", "devedor.nome"} {
		if !props[p] {
			t.Errorf("faltou a violação de %s (veio %v)", p, props)
		}
	}
	if len(c.store.Eventos()) != 0 {
		t.Fatal("publicou evento de cobrança recusada")
	}
}

func TestCriarRecusa(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()

	p := pedido()
	p.Chave = "desconhecida@example.com"
	if _, err := c.svc.Criar(ctx, p); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("chave sem conta: err = %v", err)
	}
	p.Chave = "fechada@example.com"
	if _, err := c.svc.Criar(ctx, p); !errors.Is(err, domain.ErrContaInativa) {
		t.Fatalf("conta inativa: err = %v", err)
	}
	p = pedido()
	p.Valor = "0.66" // valor mágico: o sandbox recusa
	if _, err := c.svc.Criar(ctx, p); !errors.Is(err, app.ErrPSP) {
		t.Fatalf("recusa do PSP: err = %v", err)
	}
	p.Valor = "0.00"
	if _, err := c.svc.Criar(ctx, p); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("valor zero: err = %v", err)
	}
	p = pedido()
	p.Devedor = &app.PedidoDevedor{CNPJ: "12ABC34501DE35", Nome: "Empresa"}
	if _, err := c.svc.Criar(ctx, p); err != nil {
		t.Fatalf("CNPJ alfanumérico deveria passar: %v", err)
	}
}

func TestRevisarValorERemover(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())

	valor := "20.00"
	rev, err := c.svc.Revisar(ctx, string(cob.Txid), app.RevisaoCob{Valor: &valor})
	if err != nil || rev.Valor != 2000 || rev.Revisao != 1 {
		t.Fatalf("revisão: %+v, %v", rev, err)
	}
	if p, _ := brcode.Decode(rev.BRCode); p.Valor != 2000 {
		t.Fatalf("BR Code não acompanhou o valor: %d", p.Valor)
	}
	outra := "outra@example.com"
	if _, err := c.svc.Revisar(ctx, string(cob.Txid), app.RevisaoCob{Chave: &outra}); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("troca de chave: err = %v", err)
	}
	rem, err := c.svc.Revisar(ctx, string(cob.Txid), app.RevisaoCob{Remover: true})
	if err != nil || rem.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("remoção: %+v, %v", rem, err)
	}
	if _, err := c.svc.Revisar(ctx, string(cob.Txid), app.RevisaoCob{Valor: &valor}); !errors.Is(err, domain.ErrTransicaoInvalida) {
		t.Fatalf("revisar removida: err = %v", err)
	}
	// Removida não vira paga, nem depois do atraso.
	c.clock.Avancar(time.Hour)
	if got, _ := c.svc.Consultar(ctx, string(cob.Txid)); got.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("removida virou %s", got.Status)
	}
}

func TestRevisarCobrancaJaPagaNoPSP(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	c.clock.Avancar(sandbox.AtrasoPadrao)
	valor := "1.00"
	if _, err := c.svc.Revisar(ctx, string(cob.Txid), app.RevisaoCob{Valor: &valor}); !errors.Is(err, domain.ErrTransicaoInvalida) {
		t.Fatalf("err = %v", err)
	}
}

func TestListarPaginaComoASpec(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	inicio := c.clock.Now()
	for range 5 {
		if _, err := c.svc.Criar(ctx, pedido()); err != nil {
			t.Fatal(err)
		}
		c.clock.Avancar(time.Minute)
	}
	pag, err := c.svc.Listar(ctx, app.ConsultaCobs{Inicio: inicio, Fim: c.clock.Now(), Pagina: 1, ItensPagina: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(pag.Cobs) != 2 || pag.TotalItens != 5 || pag.TotalPaginas != 3 || pag.Pagina != 1 {
		t.Fatalf("página: %d cobs, %+v", len(pag.Cobs), pag)
	}
	vazio, _ := c.svc.Listar(ctx, app.ConsultaCobs{Inicio: inicio.Add(-time.Hour), Fim: inicio})
	if len(vazio.Cobs) != 0 || vazio.TotalPaginas != 1 {
		t.Fatalf("janela vazia: %+v", vazio)
	}
	if _, err := c.svc.Listar(ctx, app.ConsultaCobs{}); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("sem janela: err = %v", err)
	}
}

func TestConsultarTxidInvalidoOuInexistente(t *testing.T) {
	c := novoCenario(t)
	if _, err := c.svc.Consultar(context.Background(), "x"); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("inválido: err = %v", err)
	}
	if _, err := c.svc.Consultar(context.Background(), "inexistente0000000000000000001"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("inexistente: err = %v", err)
	}
}

func TestAcessoRestritoAUmaContaNaoEnxergaAsOutras(t *testing.T) {
	c := novoCenario(t)
	if err := c.store.Contas().Criar(context.Background(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	daLoja := app.ComAcesso(context.Background(), app.Acesso{ContaID: "loja"})
	daOutra := app.ComAcesso(context.Background(), app.Acesso{ContaID: "outra"})

	cob, err := c.svc.Criar(daLoja, pedido())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.svc.Consultar(daOutra, string(cob.Txid)); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("outra conta enxergou a cobrança: err = %v", err)
	}
	valor := "1.00"
	if _, err := c.svc.Revisar(daOutra, string(cob.Txid), app.RevisaoCob{Valor: &valor}); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("outra conta revisou: err = %v", err)
	}
	if _, err := c.svc.Criar(daOutra, pedido()); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("outra conta cobrou na chave da loja: err = %v", err)
	}
	pag, _ := c.svc.Listar(daOutra, app.ConsultaCobs{Inicio: c.clock.Now().Add(-time.Hour), Fim: c.clock.Now().Add(time.Hour)})
	if pag.TotalItens != 0 {
		t.Fatalf("listagem vazou %d cobranças de outra conta", pag.TotalItens)
	}
	if _, err := c.svc.Consultar(daLoja, string(cob.Txid)); err != nil {
		t.Fatalf("a própria conta não enxergou: %v", err)
	}
}
