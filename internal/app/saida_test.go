package app_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	standardwebhooks "github.com/standard-webhooks/standard-webhooks/libraries/go"

	"github.com/vintemhq/vintem/internal/adapters/notifier"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/vault"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

type recebido struct {
	corpo []byte
	cab   http.Header
}

// cliente é a aplicação do cliente: responde com o status da vez e guarda o que chegou.
type cliente struct {
	srv      *httptest.Server
	mu       sync.Mutex
	chegaram []recebido
	status   atomic.Int32
}

func novoCliente(t *testing.T) *cliente {
	c := &cliente{}
	c.status.Store(http.StatusOK)
	c.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.chegaram = append(c.chegaram, recebido{corpo: b, cab: r.Header.Clone()})
		c.mu.Unlock()
		w.WriteHeader(int(c.status.Load()))
	}))
	t.Cleanup(c.srv.Close)
	return c
}

func (c *cliente) quantos() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.chegaram)
}

type cenarioSaida struct {
	saida app.Saida
	store *fake.Store
	clock *fake.Clock
}

func novaSaida(t *testing.T, agora time.Time) cenarioSaida {
	t.Helper()
	v, _ := vault.New(vault.NovaChave())
	store := fake.NewStore()
	for _, id := range []string{"loja", "outra"} {
		_ = store.Contas().Criar(context.Background(), domain.Conta{ID: id, Nome: id, PSP: "sandbox", ChavePix: id + "@example.com", Ambiente: domain.Homologacao, Ativa: true})
	}
	clock := fake.NewClock(agora)
	return cenarioSaida{saida: app.Saida{Store: store, Vault: v, Clock: clock, Enviador: notifier.NovoHTTP("test")}, store: store, clock: clock}
}

func (c cenarioSaida) evento(t *testing.T, id, conta string) {
	t.Helper()
	ev := domain.Evento{ID: id, Tipo: domain.EventoPixRecebido, ContaID: conta, OcorridoEm: c.clock.Now(), Dados: app.DadosPix{EndToEndID: "E1", ContaID: conta, Valor: "10.00"}}
	if err := c.store.Outbox().Adicionar(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
}

func TestAssinaturaVerificaComABibliotecaOficial(t *testing.T) {
	// Relógio no agora real: a lib oficial recusa timestamp com mais de 5 min de diferença.
	c := novaSaida(t, time.Now())
	cli := novoCliente(t)
	ctx := context.Background()
	_, whsec, err := c.saida.ConfigurarDestino(ctx, "loja", cli.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c.evento(t, "evt_1", "loja")
	if n, err := c.saida.Distribuir(ctx); err != nil || n != 1 {
		t.Fatalf("Distribuir = %d, %v", n, err)
	}
	r, err := c.saida.Entregar(ctx)
	if err != nil || r.Entregues != 1 || cli.quantos() != 1 {
		t.Fatalf("Entregar = %+v, %v, chegaram %d", r, err, cli.quantos())
	}
	wh, err := standardwebhooks.NewWebhook(whsec)
	if err != nil {
		t.Fatalf("o whsec não é aceito pela lib oficial: %v", err)
	}
	got := cli.chegaram[0]
	if err := wh.Verify(got.corpo, got.cab); err != nil {
		t.Fatalf("a lib oficial recusou a assinatura: %v", err)
	}
	if got.cab.Get("webhook-id") != "evt_1" || got.cab.Get("Content-Type") != "application/json" || got.cab.Get("User-Agent") != "Vintém/test" {
		t.Fatalf("cabeçalhos: %v", got.cab)
	}
	var env struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(got.corpo, &env); err != nil || env.Type != "pix.recebido" || env.Data["endToEndId"] != "E1" {
		t.Fatalf("envelope: %s", got.corpo)
	}
	// Corpo adulterado não passa.
	if err := wh.Verify(append(got.corpo, ' '), got.cab); err == nil {
		t.Fatal("corpo adulterado verificou")
	}
}

func TestRetryComBackoffAteEntregar(t *testing.T) {
	c := novaSaida(t, time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	cli := novoCliente(t)
	ctx := context.Background()
	_, _, _ = c.saida.ConfigurarDestino(ctx, "loja", cli.srv.URL)
	c.evento(t, "evt_1", "loja")
	_, _ = c.saida.Distribuir(ctx)

	cli.status.Store(http.StatusServiceUnavailable)
	if r, _ := c.saida.Entregar(ctx); r.Falhas != 1 {
		t.Fatalf("primeira tentativa: %+v", r)
	}
	pend, _ := c.store.Entregas().Listar(ctx, domain.EntregaPendente, 10)
	espera := pend[0].ProximaEm.Sub(c.clock.Now())
	if pend[0].Tentativas != 1 || pend[0].UltimoHTTP != 503 || espera < 5*time.Second || espera > 5500*time.Millisecond {
		t.Fatalf("depois da falha: %+v (espera %v)", pend[0], espera)
	}
	// Antes da hora, não tenta de novo.
	if r, _ := c.saida.Entregar(ctx); r != (app.ResultadoEntrega{}) || cli.quantos() != 1 {
		t.Fatalf("tentou antes da hora: %+v", r)
	}
	cli.status.Store(http.StatusOK)
	c.clock.Avancar(6 * time.Second)
	if r, _ := c.saida.Entregar(ctx); r.Entregues != 1 {
		t.Fatalf("segunda tentativa: %+v", r)
	}
	if bytesIguais := string(cli.chegaram[0].corpo) == string(cli.chegaram[1].corpo); !bytesIguais {
		t.Fatal("a retentativa mudou o corpo")
	}
}

func TestDLQDepoisDe24hEReenvio(t *testing.T) {
	c := novaSaida(t, time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	cli := novoCliente(t)
	cli.status.Store(http.StatusInternalServerError)
	ctx := context.Background()
	_, _, _ = c.saida.ConfigurarDestino(ctx, "loja", cli.srv.URL)
	c.evento(t, "evt_1", "loja")
	_, _ = c.saida.Distribuir(ctx)
	_, _ = c.saida.Entregar(ctx)

	c.clock.Avancar(domain.ValidadeEntrega)
	if r, _ := c.saida.Entregar(ctx); r.DLQ != 1 {
		t.Fatalf("depois de 24 h: %+v", r)
	}
	dlq, _ := c.saida.DLQ(ctx, 10)
	if len(dlq) != 1 || dlq[0].UltimoErro != "HTTP 500" || dlq[0].Tentativas != 2 {
		t.Fatalf("DLQ = %+v", dlq)
	}
	cli.status.Store(http.StatusOK)
	if n, err := c.saida.Reenviar(ctx, "evt_1"); err != nil || n != 1 {
		t.Fatalf("Reenviar = %d, %v", n, err)
	}
	if r, _ := c.saida.Entregar(ctx); r.Entregues != 1 {
		t.Fatalf("depois do reenvio: %+v", r)
	}
}

func TestDistribuiParaContaEGlobaisSoOQueCabe(t *testing.T) {
	c := novaSaida(t, time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	ctx := context.Background()
	c.evento(t, "evt_sem_destino", "loja")
	if n, _ := c.saida.Distribuir(ctx); n != 0 {
		t.Fatalf("sem destinos criou %d entregas", n)
	}
	_, _, _ = c.saida.ConfigurarDestino(ctx, "loja", "https://loja.example.com/wh")
	_, _, _ = c.saida.ConfigurarDestino(ctx, "", "https://central.example.com/wh")
	_, _, _ = c.saida.ConfigurarDestino(ctx, "outra", "https://outra.example.com/wh")
	c.evento(t, "evt_loja", "loja")
	if n, _ := c.saida.Distribuir(ctx); n != 2 {
		t.Fatalf("evento da loja gerou %d entregas, quer 2 (loja + global)", n)
	}
	// O evento antigo, sem destino na época, não volta.
	if n, _ := c.saida.Distribuir(ctx); n != 0 {
		t.Fatalf("redistribuiu %d", n)
	}
	if _, _, err := c.saida.ConfigurarDestino(ctx, "loja", "http://exemplo.com"); err == nil {
		t.Fatal("aceitou http fora de localhost")
	}
}

func TestDestinoRemovidoVaiParaDLQ(t *testing.T) {
	c := novaSaida(t, time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	ctx := context.Background()
	d, _, _ := c.saida.ConfigurarDestino(ctx, "loja", "https://loja.example.com/wh")
	c.evento(t, "evt_1", "loja")
	_, _ = c.saida.Distribuir(ctx)
	_ = c.store.Destinos().Remover(ctx, d.ID)
	if r, _ := c.saida.Entregar(ctx); r.DLQ != 1 {
		t.Fatalf("resultado: %+v", r)
	}
}

// TestAssinarBateComOSDK fixa o vetor que sdk/typescript/test/webhook.test.mjs
// usa: se a assinatura do servidor mudar, o SDK precisa mudar junto.
func TestAssinarBateComOSDK(t *testing.T) {
	corpo := `{"type":"cobranca.paga","timestamp":"2026-10-29T12:00:00Z","data":{"txid":"abc"}}`
	h := app.Assinar([]byte("chave-de-teste-do-vintem-32bytes"), "msg_1", time.Unix(1793300400, 0), []byte(corpo))
	if h["webhook-signature"] != "v1,RoW5MkBxA0viWaW7V2Jk+xNcKIoVoB13Y14TDeAScyM=" || h["webhook-timestamp"] != "1793300400" {
		t.Fatalf("cabeçalhos: %v", h)
	}
}
