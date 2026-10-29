// Package conformance é a suíte que todo adapter de PSP precisa passar (RN-51).
// Os cenários respeitam as capacidades declaradas: o que o adapter não declara
// é pulado, e o que ele declara precisa funcionar como a spec manda.
package conformance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/pkg/brcode"
)

// Alvo é o adapter sob teste e o que a suíte precisa para exercitá-lo.
type Alvo struct {
	PSP   ports.PSP
	Conta domain.Conta
	// Agora é o instante "atual" do alvo, para montar janelas de consulta.
	Agora func() time.Time
	// Pagar faz a cobrança ser paga. Só o sandbox e fixtures gravadas conseguem;
	// nil pula os cenários de pagamento.
	Pagar func(t *testing.T, txid domain.Txid)
}

// Run executa a suíte. novo deve devolver um alvo limpo a cada chamada.
func Run(t *testing.T, novo func(t *testing.T) Alvo) {
	t.Run("declara nome e capacidades", func(t *testing.T) {
		a := novo(t)
		if a.PSP.Nome() == "" {
			t.Fatal("Nome() vazio")
		}
		if len(a.PSP.Capabilities()) == 0 {
			t.Fatal("nenhuma capacidade declarada")
		}
	})

	t.Run("cob: criar devolve ATIVA, location e BR Code válido", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		c, err := a.PSP.CriarCob(context.Background(), a.Conta, novaCob(a, "conformidade0000000000001"))
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != domain.CobrancaAtiva || c.Location == "" {
			t.Fatalf("status=%s location=%q", c.Status, c.Location)
		}
		p, err := brcode.Decode(c.BRCode)
		if err != nil {
			t.Fatalf("BR Code inválido: %v", err)
		}
		if !p.Dinamico() || p.URL != c.Location {
			t.Fatalf("BR Code deveria ser dinâmico apontando para a location: %+v", p)
		}
	})

	t.Run("cob: txid repetido é recusado", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		ctx := context.Background()
		if _, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, "conformidade0000000000002")); err != nil {
			t.Fatal(err)
		}
		if _, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, "conformidade0000000000002")); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("err = %v, quer ErrJaExiste", err)
		}
	})

	t.Run("cob: consultar inexistente é ErrNaoEncontrado", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		if _, err := a.PSP.ConsultarCob(context.Background(), a.Conta, "conformidadeinexistente0001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("cob: revisar altera o valor e remover encerra", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		ctx := context.Background()
		c, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, "conformidade0000000000005"))
		if err != nil {
			t.Fatal(err)
		}
		c.Valor = 3000
		rev, err := a.PSP.RevisarCob(ctx, a.Conta, c)
		if err != nil || rev.Valor != 3000 || rev.Status != domain.CobrancaAtiva {
			t.Fatalf("revisão de valor: %+v, %v", rev, err)
		}
		rev.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
		if rev, err = a.PSP.RevisarCob(ctx, a.Conta, rev); err != nil || rev.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
			t.Fatalf("remoção: %+v, %v", rev, err)
		}
		rev.Valor = 1
		if _, err := a.PSP.RevisarCob(ctx, a.Conta, rev); err == nil {
			t.Fatal("aceitou revisar cobrança removida")
		}
	})

	t.Run("cobv: criar, consultar, revisar e remover", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCobv)
		ctx := context.Background()
		v := novaCobv(a, "conformidadev000000000000001")
		c, err := a.PSP.CriarCobv(ctx, a.Conta, v)
		if err != nil {
			t.Fatal(err)
		}
		if c.Status != domain.CobrancaAtiva || c.Location == "" || c.Vencimento != v.Vencimento {
			t.Fatalf("criada: %+v", c)
		}
		if p, err := brcode.Decode(c.BRCode); err != nil || p.URL != c.Location {
			t.Fatalf("BR Code: %+v, %v", p, err)
		}
		if _, err := a.PSP.CriarCobv(ctx, a.Conta, v); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("txid repetido: err = %v", err)
		}
		if _, err := a.PSP.ConsultarCobv(ctx, a.Conta, "conformidadevinexistente0001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: err = %v", err)
		}
		c.Valor = 20000
		rev, err := a.PSP.RevisarCobv(ctx, a.Conta, c)
		if err != nil || rev.Valor != 20000 {
			t.Fatalf("revisão: %+v, %v", rev, err)
		}
		rev.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
		if rev, err = a.PSP.RevisarCobv(ctx, a.Conta, rev); err != nil || rev.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
			t.Fatalf("remoção: %+v, %v", rev, err)
		}
	})

	t.Run("pagamento: cobv paga dentro da validade vira CONCLUIDA", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCobv)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		c, err := a.PSP.CriarCobv(ctx, a.Conta, novaCobv(a, "conformidadev000000000000002"))
		if err != nil {
			t.Fatal(err)
		}
		a.Pagar(t, c.Txid)
		if got, err := a.PSP.ConsultarCobv(ctx, a.Conta, c.Txid); err != nil || got.Status != domain.CobrancaConcluida {
			t.Fatalf("depois de pagar: %s, %v", got.Status, err)
		}
	})

	t.Run("pagamento: cob removida não é paga", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		c, _ := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, "conformidade0000000000006"))
		c.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
		if _, err := a.PSP.RevisarCob(ctx, a.Conta, c); err != nil {
			t.Fatal(err)
		}
		a.Pagar(t, c.Txid)
		if got, _ := a.PSP.ConsultarCob(ctx, a.Conta, c.Txid); got.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
			t.Fatalf("cobrança removida virou %s", got.Status)
		}
	})

	t.Run("pagamento: cob paga vira CONCLUIDA e aparece em ListarPix", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapCob)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		inicio := a.Agora()
		c, err := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, "conformidade0000000000003"))
		if err != nil {
			t.Fatal(err)
		}
		a.Pagar(t, c.Txid)
		got, err := a.PSP.ConsultarCob(ctx, a.Conta, c.Txid)
		if err != nil || got.Status != domain.CobrancaConcluida {
			t.Fatalf("depois de pagar: status=%s err=%v", got.Status, err)
		}
		pix, err := a.PSP.ListarPix(ctx, a.Conta, ports.Janela{Inicio: inicio, Fim: a.Agora().Add(time.Second)})
		if err != nil {
			t.Fatal(err)
		}
		if len(pix) != 1 || pix[0].Txid != c.Txid || pix[0].Valor != c.Valor || len(pix[0].EndToEndID) != 32 {
			t.Fatalf("ListarPix = %+v", pix)
		}
	})

	t.Run("devolução: não passa do valor recebido e é idempotente pelo id", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapDevolucao)
		if a.Pagar == nil {
			t.Skip("alvo não simula pagamento")
		}
		ctx := context.Background()
		inicio := a.Agora()
		c, _ := a.PSP.CriarCob(ctx, a.Conta, novaCob(a, "conformidade0000000000004"))
		a.Pagar(t, c.Txid)
		pix, _ := a.PSP.ListarPix(ctx, a.Conta, ports.Janela{Inicio: inicio, Fim: a.Agora().Add(time.Second)})
		if len(pix) == 0 {
			t.Fatal("pagamento não apareceu")
		}
		e2e := pix[0].EndToEndID

		d, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 1000})
		if err != nil || d.Status == "" {
			t.Fatalf("primeira devolução: %+v, %v", d, err)
		}
		if _, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d1", EndToEndID: e2e, Valor: 1000}); err != nil {
			t.Fatalf("repetir o mesmo id deveria ser idempotente: %v", err)
		}
		if _, err := a.PSP.Devolver(ctx, a.Conta, domain.Devolucao{ID: "d2", EndToEndID: e2e, Valor: c.Valor}); err == nil {
			t.Fatal("aceitou devolver mais do que o recebido")
		}
	})

	t.Run("webhook: configurar URL", func(t *testing.T) {
		a := novo(t)
		exigir(t, a, ports.CapWebhook)
		if err := a.PSP.ConfigurarWebhook(context.Background(), a.Conta, "https://lastro.example.com/webhook/"+a.Conta.ID); err != nil {
			t.Fatal(err)
		}
	})
}

func exigir(t *testing.T, a Alvo, c ports.Capacidade) {
	t.Helper()
	if err := a.PSP.Capabilities().Suporta(c); err != nil {
		t.Skipf("adapter não declara %s", c)
	}
}

func novaCobv(a Alvo, txid string) domain.CobrancaVencimento {
	return domain.CobrancaVencimento{
		Txid: domain.Txid(txid), ContaID: a.Conta.ID, Valor: 15000, Vencimento: domain.DataDe(a.Agora()).SomarDias(10),
		ValidadeAposVencimento: 30, CriadaEm: a.Agora(),
		Devedor: domain.DevedorCompleto{Devedor: domain.Devedor{Documento: "11222333000181", Nome: "Empresa de Teste"}},
		Multa:   &domain.ValorModalidade{Modalidade: 2, ValorPerc: 200},
	}
}

func novaCob(a Alvo, txid string) domain.Cobranca {
	return domain.Cobranca{
		Txid: domain.Txid(txid), ContaID: a.Conta.ID, Valor: 2500, Expiracao: domain.ExpiracaoPadrao,
		CriadaEm: a.Agora(), SolicitacaoPagador: "Teste de conformidade",
	}
}
