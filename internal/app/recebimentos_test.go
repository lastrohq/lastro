package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
)

func pixDe(e2e, txid string, valor domain.Money) domain.PixRecebido {
	return domain.PixRecebido{EndToEndID: e2e, ContaID: "loja", Txid: domain.Txid(txid), Valor: valor, Horario: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)}
}

func eventosDoTipo(evs []domain.Evento, tipo domain.TipoEvento) []domain.Evento {
	var out []domain.Evento
	for _, e := range evs {
		if e.Tipo == tipo {
			out = append(out, e)
		}
	}
	return out
}

func TestRecebimentoConcluiACobUmaVezSo(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}

	p := pixDe("E12345678202610291200abcdefghijk", string(cob.Txid), cob.Valor)
	for range 2 { // o mesmo Pix avisado duas vezes
		if _, err := rec.Registrar(ctx, []domain.PixRecebido{p}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid)
	if got.Status != domain.CobrancaConcluida || got.Revisao != 1 {
		t.Fatalf("cob: %s rev %d", got.Status, got.Revisao)
	}
	evs := c.store.Eventos()
	if n := len(eventosDoTipo(evs, domain.EventoPixRecebido)); n != 1 {
		t.Fatalf("%d eventos pix.recebido, quer 1", n)
	}
	if n := len(eventosDoTipo(evs, domain.EventoCobrancaConcluida)); n != 1 {
		t.Fatalf("%d eventos cobranca.concluida, quer 1", n)
	}
	pixes, _ := rec.DaCobranca(ctx, "loja", cob.Txid)
	if len(pixes) != 1 {
		t.Fatalf("DaCobranca = %d", len(pixes))
	}
}

func TestRecebimentoComValorDivergente(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}
	_, _ = rec.Registrar(ctx, []domain.PixRecebido{pixDe("E12345678202610291200abcdefghijk", string(cob.Txid), cob.Valor-1)})
	ev := eventosDoTipo(c.store.Eventos(), domain.EventoPixRecebido)[0]
	if !ev.Dados.(app.DadosPix).Divergente {
		t.Fatal("valor diferente não foi marcado como divergente")
	}
	if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("o banco liquidou: a cob deveria concluir mesmo divergente, está %s", got.Status)
	}
}

func TestRecebimentoNaoConcluiCobDeOutraContaNemSemTxid(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	cob, _ := c.svc.Criar(ctx, pedido())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}
	alheio := pixDe("E12345678202610291200abcdefghijk", string(cob.Txid), cob.Valor)
	alheio.ContaID = "fechada"
	semTxid := pixDe("E12345678202610291201abcdefghijk", "", 500)
	if n, err := rec.Registrar(ctx, []domain.PixRecebido{alheio, semTxid}); err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid); got.Status != domain.CobrancaAtiva {
		t.Fatalf("Pix de outra conta concluiu a cob: %s", got.Status)
	}
}

func TestRecebimentoConcluiCobv(t *testing.T) {
	c := novoCenarioV(t)
	ctx := context.Background()
	v, _ := c.svc.Criar(ctx, pedidoV())
	rec := app.Recebimentos{Store: c.store, Clock: c.clock}
	_, _ = rec.Registrar(ctx, []domain.PixRecebido{pixDe("E12345678202610291200abcdefghijk", string(v.Txid), 14000)})
	if got, _ := c.store.CobrancasV().BuscarPorTxid(ctx, v.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("cobv: %s", got.Status)
	}
}

func TestWebhookConfigurarEReceber(t *testing.T) {
	c := novoCenario(t)
	ctx := context.Background()
	reg := psp.NewRegistry(sandbox.New(c.clock))
	wh := app.Webhooks{Store: c.store, PSPs: reg, Recebimentos: app.Recebimentos{Store: c.store, Clock: c.clock}}

	if _, err := wh.Configurar(ctx, "loja", "http://lastro.example.com"); !errors.Is(err, app.ErrRequisicaoInvalida) {
		t.Fatalf("http fora de localhost: err = %v", err)
	}
	endereco, err := wh.Configurar(ctx, "loja", "https://lastro.example.com/")
	if err != nil || !strings.HasPrefix(endereco, "https://lastro.example.com/webhooks/pix/loja/") {
		t.Fatalf("Configurar = %q, %v", endereco, err)
	}
	token := endereco[strings.LastIndex(endereco, "/")+1:]

	cob, _ := c.svc.Criar(ctx, pedido())
	corpo := `{"pix":[{"endToEndId":"E12345678202610291200abcdefghijk","txid":"` + string(cob.Txid) + `","valor":"10.50","horario":"2026-10-29T12:00:00Z"}]}`
	if _, err := wh.Receber(ctx, "loja", "token-errado", []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("token errado: err = %v", err)
	}
	if _, err := wh.Receber(ctx, "loja", token, []byte(`{"pix":[]}`)); !errors.Is(err, domain.ErrPixInvalido) {
		t.Fatalf("corpo inválido: err = %v", err)
	}
	if n, err := wh.Receber(ctx, "loja", token, []byte(corpo)); err != nil || n != 1 {
		t.Fatalf("Receber: n=%d err=%v", n, err)
	}
	if got, _ := c.store.Cobrancas().BuscarPorTxid(ctx, cob.Txid); got.Status != domain.CobrancaConcluida {
		t.Fatalf("cob: %s", got.Status)
	}
	if _, err := wh.Receber(ctx, "fechada", token, []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("token de outra conta: err = %v", err)
	}
	// Reconfigurar troca o token: o antigo para de valer.
	_, _ = wh.Configurar(ctx, "loja", "https://lastro.example.com")
	if _, err := wh.Receber(ctx, "loja", token, []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatal("token antigo continuou valendo")
	}
}
