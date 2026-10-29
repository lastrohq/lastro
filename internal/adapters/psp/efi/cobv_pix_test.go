package efi_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/pkg/brcode"
)

func cobv(txid string) domain.CobrancaVencimento {
	return domain.CobrancaVencimento{
		Txid: domain.Txid(txid), Chave: "loja@example.com", Valor: 12345,
		Multa:      &domain.ValorModalidade{Modalidade: 2, ValorPerc: 1500},
		Juros:      &domain.ValorModalidade{Modalidade: 2, ValorPerc: 200},
		Desconto:   &domain.Desconto{Modalidade: 1, DatasFixas: []domain.DescontoData{{Data: domain.NovaData(2026, 11, 20), ValorPerc: 3000}}},
		Vencimento: domain.NovaData(2026, 11, 30), ValidadeAposVencimento: 30,
		Devedor: domain.DevedorCompleto{Devedor: domain.Devedor{Documento: "12345678909", Nome: "Francisco da Silva"},
			Logradouro: "Alameda Souza, Numero 80", Cidade: "Recife", UF: "PE", CEP: "70011750"},
		SolicitacaoPagador: "Cobrança dos serviços prestados.",
	}
}

func TestCobvCriarConsultarRevisarERemover(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	quer := cobv("efiv0000000000000000000001")
	criada, err := c.psp.CriarCobv(ctx, c.conta, quer)
	if err != nil {
		t.Fatal(err)
	}
	if criada.Status != domain.CobrancaAtiva || criada.Location == "" || criada.Vencimento != quer.Vencimento {
		t.Fatalf("criada = %+v", criada)
	}
	if p, err := brcode.Decode(criada.BRCode); err != nil || p.URL != criada.Location {
		t.Fatalf("BR Code: %v", err)
	}

	lida, err := c.psp.ConsultarCobv(ctx, c.conta, quer.Txid)
	if err != nil {
		t.Fatal(err)
	}
	// Tudo o que foi enviado volta igual: multa, juros, desconto por data, devedor e validade.
	for nome, par := range map[string][2]any{
		"valor": {lida.Valor, quer.Valor}, "multa": {*lida.Multa, *quer.Multa}, "juros": {*lida.Juros, *quer.Juros},
		"desconto": {*lida.Desconto, *quer.Desconto}, "devedor": {lida.Devedor, quer.Devedor},
		"validade": {lida.ValidadeAposVencimento, quer.ValidadeAposVencimento}, "solicitação": {lida.SolicitacaoPagador, quer.SolicitacaoPagador},
	} {
		if !reflect.DeepEqual(par[0], par[1]) {
			t.Errorf("%s: lido %+v, enviado %+v", nome, par[0], par[1])
		}
	}

	lida.Valor = 20000
	rev, err := c.psp.RevisarCobv(ctx, c.conta, lida)
	if err != nil || rev.Valor != 20000 || rev.Revisao != 1 {
		t.Fatalf("revisar: %+v, %v", rev, err)
	}
	rev.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
	if rev, err = c.psp.RevisarCobv(ctx, c.conta, rev); err != nil || rev.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("remover: %+v, %v", rev, err)
	}
}

func TestCobvErros(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	if _, err := c.psp.CriarCobv(ctx, c.conta, cobv("efiv0000000000000000000002")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.psp.CriarCobv(ctx, c.conta, cobv("efiv0000000000000000000002")); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("txid repetido: err = %v; quer ErrJaExiste", err)
	}
	if _, err := c.psp.ConsultarCobv(ctx, c.conta, "efivinexistente000000000001"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("inexistente: err = %v; quer ErrNaoEncontrado", err)
	}
	// 400 num txid que não existe: é o erro de validação, não duplicidade.
	c.efi.Falhar(http.StatusBadRequest)
	if _, err := c.psp.CriarCobv(ctx, c.conta, cobv("efiv0000000000000000000003")); err == nil || errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("400 de validação virou %v", err)
	}
}

func TestListarPixPaginaERespeitaAJanela(t *testing.T) {
	c := novo(t)
	inicio := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	c.efi.AdicionarPix(250, inicio) // de 10:00:00 a 10:04:09, um por segundo
	fim := inicio.Add(249 * time.Second)

	pix, err := c.psp.ListarPix(context.Background(), c.conta, ports.Janela{Inicio: inicio, Fim: fim})
	if err != nil {
		t.Fatal(err)
	}
	// 250 no Efí em 3 páginas; o último cai exatamente em Fim e fica para a próxima janela.
	if len(pix) != 249 {
		t.Fatalf("%d Pix; quer 249", len(pix))
	}
	vistos := map[string]bool{}
	for _, p := range pix {
		if vistos[p.EndToEndID] || p.ContaID != "loja" || p.Valor != 100 || p.Pagador == nil || p.Pagador.Documento != "12345678909" {
			t.Fatalf("Pix inesperado: %+v", p)
		}
		vistos[p.EndToEndID] = true
	}
	paginas := 0
	for _, ch := range c.efi.Chamadas() {
		if ch == "GET /v2/pix" {
			paginas++
		}
	}
	if paginas != 3 {
		t.Fatalf("%d páginas pedidas; quer 3", paginas)
	}
}

func TestPixDeCobrancaPagaApareceComOTxid(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	criada, _ := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000010"))
	horario := time.Date(2026, 10, 29, 12, 5, 0, 0, time.UTC)
	e2e := c.efi.Pagar(string(criada.Txid), horario)

	pix, err := c.psp.ListarPix(ctx, c.conta, ports.Janela{Inicio: horario.Add(-time.Minute), Fim: horario.Add(time.Minute)})
	if err != nil || len(pix) != 1 {
		t.Fatalf("ListarPix = %+v, %v", pix, err)
	}
	if pix[0].EndToEndID != e2e || pix[0].Txid != criada.Txid || pix[0].Valor != criada.Valor || !pix[0].Horario.Equal(horario) {
		t.Fatalf("Pix = %+v", pix[0])
	}
	if lida, _ := c.psp.ConsultarCob(ctx, c.conta, criada.Txid); lida.Status != domain.CobrancaConcluida {
		t.Fatalf("cob paga ficou %s", lida.Status)
	}
}

func TestDevolverEIdempotentePeloIDENaoPassaDoRecebido(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	criada, _ := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000011")) // R$ 10,50
	e2e := c.efi.Pagar(string(criada.Txid), agora)

	d, err := c.psp.Devolver(ctx, c.conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 500})
	if err != nil || d.Status != domain.DevolucaoEmProcessamento || d.Valor != 500 || d.CriadaEm.IsZero() {
		t.Fatalf("devolver: %+v, %v", d, err)
	}
	// Mesmo id e mesmo valor: o Efí responde 409, o adapter devolve a existente.
	repetida, err := c.psp.Devolver(ctx, c.conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 500})
	if err != nil || repetida.ID != "d1" || repetida.Valor != 500 {
		t.Fatalf("repetir d1: %+v, %v", repetida, err)
	}
	if _, err := c.psp.Devolver(ctx, c.conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 600}); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("d1 com outro valor: err = %v; quer ErrJaExiste", err)
	}
	if _, err := c.psp.Devolver(ctx, c.conta, domain.Devolucao{ID: "d2", EndToEndID: e2e, Valor: criada.Valor}); err == nil {
		t.Fatal("aceitou devolver mais do que o recebido")
	}
	if _, err := c.psp.Devolver(ctx, c.conta, domain.Devolucao{ID: "d3", EndToEndID: "E00000000000000000000000000000ZZ", Valor: 100}); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("Pix inexistente: err = %v; quer ErrNaoEncontrado", err)
	}
}
