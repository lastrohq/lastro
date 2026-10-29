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
)

type cenarioV struct {
	svc   app.CobrancasV
	cob   app.Cobrancas
	store *fake.Store
	clock *fake.Clock
}

// 29/10/2026, 10h em Brasília.
var agoraV = time.Date(2026, 10, 29, 13, 0, 0, 0, time.UTC)

func novoCenarioV(t *testing.T) cenarioV {
	t.Helper()
	clock := fake.NewClock(agoraV)
	store := fake.NewStore()
	_ = store.Contas().Criar(context.Background(), domain.Conta{ID: "loja", Nome: "Loja", PSP: "sandbox", ChavePix: "loja@example.com", Ambiente: domain.Homologacao, Ativa: true})
	reg := psp.NewRegistry(sandbox.New(clock))
	return cenarioV{
		svc: app.CobrancasV{Store: store, PSPs: reg, Clock: clock}, cob: app.Cobrancas{Store: store, PSPs: reg, Clock: clock},
		store: store, clock: clock,
	}
}

func pedidoV() app.PedidoCobv {
	return app.PedidoCobv{
		Txid: "boleto000000000000000000001", Chave: "loja@example.com", Valor: "150.00", Vencimento: "2026-11-30",
		Devedor: &app.PedidoDevedorCompleto{PedidoDevedor: app.PedidoDevedor{CNPJ: "11222333000181", Nome: "Empresa"}, UF: "PE", CEP: "50000000"},
		Multa:   &app.PedidoModalidade{Modalidade: 2, ValorPerc: "2.00"},
		Juros:   &app.PedidoModalidade{Modalidade: 3, ValorPerc: "1.00"},
		Desconto: &app.PedidoDesconto{Modalidade: 1, DatasFixas: []app.PedidoDescontoData{
			{Data: "2026-11-20", ValorPerc: "10.00"}, {Data: "2026-11-25", ValorPerc: "5.00"},
		}},
	}
}

func props(t *testing.T, err error) map[string]bool {
	t.Helper()
	var ev *app.ErroValidacao
	if !errors.As(err, &ev) {
		t.Fatalf("err = %v, quer ErroValidacao", err)
	}
	out := map[string]bool{}
	for _, v := range ev.Violacoes {
		out[v.Propriedade] = true
	}
	return out
}

func TestCobvFluxoCompleto(t *testing.T) {
	c := novoCenarioV(t)
	ctx := context.Background()
	v, err := c.svc.Criar(ctx, pedidoV())
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != domain.CobrancaAtiva || v.Valor != 15000 || v.ValidadeAposVencimento != 30 || v.Vencimento.String() != "2026-11-30" ||
		v.Multa == nil || v.Multa.ValorPerc != 200 || v.Desconto == nil || len(v.Desconto.DatasFixas) != 2 || v.Devedor.UF != "PE" || v.BRCode == "" {
		t.Fatalf("criada: %+v", v)
	}
	c.clock.Avancar(sandbox.AtrasoPadrao)
	got, err := c.svc.Consultar(ctx, string(v.Txid))
	if err != nil || got.Status != domain.CobrancaConcluida || got.Revisao != 1 {
		t.Fatalf("depois de pagar: %+v, %v", got, err)
	}
	evs := c.store.Eventos()
	if len(evs) != 2 || evs[0].Dados.(app.DadosCobranca).Tipo != "cobv" {
		t.Fatalf("eventos = %+v", evs)
	}
}

func TestCobvTxidCompartilhadoComCob(t *testing.T) {
	c := novoCenarioV(t)
	ctx := context.Background()
	p := pedido()
	p.Txid = "boleto000000000000000000001"
	if _, err := c.cob.Criar(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := c.svc.Criar(ctx, pedidoV()); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("cobv com txid de cob: err = %v", err)
	}
}

func TestCobvRegrasDaRN14(t *testing.T) {
	c := novoCenarioV(t)
	ctx := context.Background()
	casos := map[string]struct {
		mudar func(*app.PedidoCobv)
		prop  string
	}{
		"sem devedor":           {func(p *app.PedidoCobv) { p.Devedor = nil }, "devedor"},
		"vencimento no passado": {func(p *app.PedidoCobv) { p.Vencimento = "2026-10-28" }, "calendario.dataDeVencimento"},
		"vencimento inválido":   {func(p *app.PedidoCobv) { p.Vencimento = "30/11/2026" }, "calendario.dataDeVencimento"},
		"validade negativa":     {func(p *app.PedidoCobv) { n := -1; p.ValidadeDias = &n }, "calendario.validadeAposVencimento"},
		"multa modalidade 3":    {func(p *app.PedidoCobv) { p.Multa.Modalidade = 3 }, "valor.multa.modalidade"},
		"multa acima de 100%":   {func(p *app.PedidoCobv) { p.Multa.ValorPerc = "100.01" }, "valor.multa.valorPerc"},
		"juros modalidade 9":    {func(p *app.PedidoCobv) { p.Juros.Modalidade = 9 }, "valor.juros.modalidade"},
		"abatimento >= valor": {func(p *app.PedidoCobv) {
			p.Abatimento = &app.PedidoModalidade{Modalidade: 1, ValorPerc: "150.00"}
		}, "valor.abatimento.valorPerc"},
		"desconto depois do vencimento": {func(p *app.PedidoCobv) { p.Desconto.DatasFixas[1].Data = "2026-12-01" }, "valor.desconto.descontoDataFixa[1].data"},
		"desconto com data repetida":    {func(p *app.PedidoCobv) { p.Desconto.DatasFixas[1].Data = "2026-11-20" }, "valor.desconto.descontoDataFixa[1].data"},
		"quatro datas de desconto": {func(p *app.PedidoCobv) {
			p.Desconto.DatasFixas = append(p.Desconto.DatasFixas, app.PedidoDescontoData{Data: "2026-11-26", ValorPerc: "1.00"}, app.PedidoDescontoData{Data: "2026-11-27", ValorPerc: "1.00"})
		}, "valor.desconto.descontoDataFixa"},
		"antecipação sem valorPerc": {func(p *app.PedidoCobv) { p.Desconto = &app.PedidoDesconto{Modalidade: 3} }, "valor.desconto.valorPerc"},
		"antecipação com datas": {func(p *app.PedidoCobv) {
			vp := "1.00"
			p.Desconto = &app.PedidoDesconto{Modalidade: 5, ValorPerc: &vp, DatasFixas: []app.PedidoDescontoData{{Data: "2026-11-20", ValorPerc: "1.00"}}}
		}, "valor.desconto.descontoDataFixa"},
		"uf minúscula":  {func(p *app.PedidoCobv) { p.Devedor.UF = "pe" }, "devedor.uf"},
		"cep com traço": {func(p *app.PedidoCobv) { p.Devedor.CEP = "50000-000" }, "devedor.cep"},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			p := pedidoV()
			caso.mudar(&p)
			_, err := c.svc.Criar(ctx, p)
			if got := props(t, err); !got[caso.prop] {
				t.Fatalf("faltou %s, veio %v", caso.prop, got)
			}
		})
	}
}

func TestCobvDescontoPorAntecipacaoValido(t *testing.T) {
	c := novoCenarioV(t)
	p := pedidoV()
	vp := "0.50"
	p.Desconto = &app.PedidoDesconto{Modalidade: 5, ValorPerc: &vp}
	v, err := c.svc.Criar(context.Background(), p)
	if err != nil || v.Desconto.ValorPerc != 50 || len(v.Desconto.DatasFixas) != 0 {
		t.Fatalf("%+v, %v", v.Desconto, err)
	}
}

func TestCobvRevisarERemover(t *testing.T) {
	c := novoCenarioV(t)
	ctx := context.Background()
	v, _ := c.svc.Criar(ctx, pedidoV())

	novoVenc, valor := "2026-12-15", "200.00"
	rev, err := c.svc.Revisar(ctx, string(v.Txid), app.RevisaoCobv{Vencimento: &novoVenc, Valor: &valor})
	if err != nil || rev.Vencimento.String() != "2026-12-15" || rev.Valor != 20000 || rev.Revisao != 1 || rev.Multa == nil {
		t.Fatalf("revisão: %+v, %v", rev, err)
	}
	// Desconto antes do novo vencimento continua válido; abatimento maior que o valor não.
	abat := &app.PedidoModalidade{Modalidade: 1, ValorPerc: "250.00"}
	if _, err := c.svc.Revisar(ctx, string(v.Txid), app.RevisaoCobv{Abatimento: abat}); !props(t, err)["valor.abatimento.valorPerc"] {
		t.Fatal("aceitou abatimento maior que o valor revisado")
	}
	rem, err := c.svc.Revisar(ctx, string(v.Txid), app.RevisaoCobv{Remover: true})
	if err != nil || rem.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("remoção: %+v, %v", rem, err)
	}
	c.clock.Avancar(time.Hour)
	if got, _ := c.svc.Consultar(ctx, string(v.Txid)); got.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("removida virou %s", got.Status)
	}
}

func TestCobvAcessoRestrito(t *testing.T) {
	c := novoCenarioV(t)
	_ = c.store.Contas().Criar(context.Background(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true})
	v, _ := c.svc.Criar(context.Background(), pedidoV())
	daOutra := app.ComAcesso(context.Background(), app.Acesso{ContaID: "outra"})
	if _, err := c.svc.Consultar(daOutra, string(v.Txid)); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("err = %v", err)
	}
	pag, _ := c.svc.Listar(daOutra, app.ConsultaCobs{Inicio: agoraV.Add(-time.Hour), Fim: agoraV.Add(time.Hour)})
	if pag.TotalItens != 0 {
		t.Fatalf("listagem vazou %d", pag.TotalItens)
	}
}
