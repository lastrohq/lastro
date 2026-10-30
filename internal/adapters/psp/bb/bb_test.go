package bb_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/bb"
	"github.com/vintemhq/vintem/internal/adapters/psp/bb/bbtest"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
	"github.com/vintemhq/vintem/test/conformance"
	"github.com/vintemhq/vintem/test/conformance/fita"
)

const (
	clientID       = "client-id-bb"
	clientSecret   = "client-secret-que-nao-pode-vazar"
	chaveAplicacao = "chave-de-aplicacao-do-portal"
)

var agora = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type credenciais map[string]domain.Credenciais

func (c credenciais) Credenciais(_ context.Context, id string) (domain.Credenciais, error) {
	cred, ok := c[id]
	if !ok {
		return domain.Credenciais{}, ports.ErrNaoEncontrado
	}
	return cred, nil
}

type cenario struct {
	psp   *bb.PSP
	srv   *bbtest.Servidor
	conta domain.Conta
}

// novo monta o adapter contra o BB falso na homologação: sem certificado,
// como a homologação do BB.
func novo(t *testing.T, relogio ports.Clock) cenario {
	t.Helper()
	srv := bbtest.Novo(t, nil, clientID, clientSecret, chaveAplicacao)
	cred := credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, ChaveAplicacao: chaveAplicacao}}
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "bb", ChavePix: "loja@example.com", Ambiente: domain.Homologacao, Ativa: true}
	p := bb.New(cred, relogio, bb.ComURLs("https://producao.invalid/pix/v2", srv.URL), bb.ComRaizes(srv.Raizes))
	return cenario{psp: p, srv: srv, conta: conta}
}

// A suíte de conformidade contra o BB falso, na homologação.
func TestConformidade(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		relogio := fake.NewClock(agora)
		c := novo(t, relogio)
		return conformance.Alvo{
			PSP: c.psp, Conta: c.conta, Agora: relogio.Now,
			Pagar: func(_ *testing.T, txid domain.Txid) {
				c.srv.Pagar(string(txid), relogio.Now())
				relogio.Avancar(time.Second)
			},
		}
	})
}

// Em produção o BB exige mTLS e a chave vai em gw-app-key.
func TestProducaoComMTLSEGwAppKey(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := bbtest.Novo(t, par.Certificado, clientID, clientSecret, chaveAplicacao)
	conta := domain.Conta{ID: "loja", PSP: "bb", ChavePix: "loja@example.com", Ambiente: domain.Producao, Ativa: true}
	ctx := context.Background()

	semCert := bb.New(credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, ChaveAplicacao: chaveAplicacao}}, fake.NewClock(agora),
		bb.ComURLs(srv.URL, "https://homologacao.invalid/pix/v2"), bb.ComRaizes(srv.Raizes))
	if _, err := semCert.ConsultarCob(ctx, conta, "x00000000000000000000000001"); err == nil || !strings.Contains(err.Error(), "client id, client secret e certificado") {
		t.Fatalf("produção sem certificado: %v", err)
	}

	cred := credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, ChaveAplicacao: chaveAplicacao, Certificado: par.PEM}}
	p := bb.New(cred, fake.NewClock(agora), bb.ComURLs(srv.URL, "https://homologacao.invalid/pix/v2"), bb.ComRaizes(srv.Raizes))
	cob, err := p.CriarCob(ctx, conta, domain.Cobranca{Txid: "producao000000000000000001", Valor: 1, Expiracao: time.Hour, Chave: conta.ChavePix})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cob.BRCode, "000201") {
		t.Fatalf("BR Code de textoImagemQRcode: %q", cob.BRCode)
	}
	for _, nome := range srv.ParametrosChave() {
		if nome != "gw-app-key" {
			t.Fatalf("em produção a chave vai em gw-app-key, veio em %s", nome)
		}
	}
}

func TestHomologacaoUsaGwDevAppKey(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	if _, err := c.psp.ConsultarCob(context.Background(), c.conta, "inexistente00000000000000zz"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatal(err)
	}
	if got := c.srv.ParametrosChave(); !slices.Equal(got, []string{"gw-dev-app-key"}) {
		t.Fatalf("parâmetros: %v", got)
	}
}

func TestTokenEmBasicComEscoposEReusado(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	ctx := context.Background()
	for range 3 {
		if _, err := c.psp.ConsultarCob(ctx, c.conta, "inexistente00000000000000zz"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("consulta: %v", err)
		}
	}
	if n := c.srv.TokensEmitidos(); n != 1 {
		t.Fatalf("%d tokens emitidos, quer 1", n)
	}
	if got := c.srv.EscoposPedidos(); got != bb.Escopos {
		t.Fatalf("escopos pedidos: %q", got)
	}
}

func TestSemChaveAplicacaoExplica(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	p := bb.New(credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret}}, fake.NewClock(agora),
		bb.ComURLs("https://producao.invalid/pix/v2", c.srv.URL), bb.ComRaizes(c.srv.Raizes))
	if _, err := p.ConsultarCob(context.Background(), c.conta, "x00000000000000000000000001"); !errors.Is(err, bb.ErrSemChaveAplicacao) {
		t.Fatalf("err = %v", err)
	}
}

func TestCredencialErradaExplicaOQueConferir(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	p := bb.New(credenciais{"loja": {ClientID: clientID, ClientSecret: "errado", ChaveAplicacao: chaveAplicacao}}, fake.NewClock(agora),
		bb.ComURLs("https://producao.invalid/pix/v2", c.srv.URL), bb.ComRaizes(c.srv.Raizes))
	_, err := p.ConsultarCob(context.Background(), c.conta, "x00000000000000000000000001")
	if err == nil || !strings.Contains(err.Error(), "invalid_client") || !strings.Contains(err.Error(), "escopos da aplicação") {
		t.Fatalf("err = %v", err)
	}
}

func TestChaveAplicacaoErradaRecusadaPeloGateway(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	p := bb.New(credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, ChaveAplicacao: "outra"}}, fake.NewClock(agora),
		bb.ComURLs("https://producao.invalid/pix/v2", c.srv.URL), bb.ComRaizes(c.srv.Raizes))
	_, err := p.ConsultarCob(context.Background(), c.conta, "x00000000000000000000000001")
	if !errors.Is(err, bb.ErrBB) || !strings.Contains(err.Error(), "gw-dev-app-key ausente ou inválida") {
		t.Fatalf("err = %v", err)
	}
}

func TestErrosNosFormatosDoBB(t *testing.T) {
	casos := map[string]string{
		`{"type":"https://pix.bcb.gov.br/api/v2/error/CobOperacaoInvalida","detail":"valor inválido"}`: "CobOperacaoInvalida: valor inválido",
		`{"erros":[{"codigo":"4769515","mensagem":"chave não pertence ao recebedor"}]}`:                "status 400: chave não pertence ao recebedor",
		`{"errors":[{"code":"401","message":"invalid app key"}]}`:                                      "status 400: invalid app key",
		`{"statusCode":400,"error":"Bad Request","message":"corpo inválido"}`:                          "status 400: corpo inválido",
	}
	for corpo, quer := range casos {
		if err := bb.Traduzir(400, []byte(corpo)); !errors.Is(err, bb.ErrBB) || !strings.Contains(err.Error(), quer) {
			t.Errorf("%s: %v", corpo, err)
		}
	}
	if err := bb.Traduzir(404, nil); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("404: %v", err)
	}
}

func TestSimularPagamentoNaHomologacao(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	ctx := context.Background()
	cob, err := c.psp.CriarCob(ctx, c.conta, domain.Cobranca{Txid: "simulacao00000000000000001", Valor: 1050, Expiracao: time.Hour, Chave: c.conta.ChavePix})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.psp.SimularPagamento(ctx, c.conta, cob.BRCode); err != nil {
		t.Fatal(err)
	}
	if lida, err := c.psp.ConsultarCob(ctx, c.conta, cob.Txid); err != nil || lida.Status != domain.CobrancaConcluida {
		t.Fatalf("depois de simular: %s %v", lida.Status, err)
	}
	producao := c.conta
	producao.Ambiente = domain.Producao
	if err := c.psp.SimularPagamento(ctx, producao, cob.BRCode); !errors.Is(err, bb.ErrSoNaHomologacao) {
		t.Fatalf("em produção: %v", err)
	}
}

func TestWebhookRegistradoNaChave(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	if err := c.psp.ConfigurarWebhook(context.Background(), c.conta, "https://vintem.example.com/webhooks/pix/loja/token"); err != nil {
		t.Fatal(err)
	}
	if got := c.srv.Webhook("loja@example.com"); got != "https://vintem.example.com/webhooks/pix/loja/token" {
		t.Fatalf("webhook: %q", got)
	}
}

const dirFitas = "../../../../test/fixtures/bb"

// ChavePixHomologacao é a chave EVP de teste que o BB publica para a
// homologação, compartilhada por todas as aplicações.
const ChavePixHomologacao = "9e881f18-cc66-4fc7-8f2c-a795dbb2bfc1"

// TestConformidadeGravada roda a suíte contra as fitas da homologação do BB
// (RN-51). Para gravar, com uma aplicação "Em teste" do Portal Developers BB:
//
//	VINTEM_GRAVAR_BB=1 BB_CLIENT_ID=... BB_CLIENT_SECRET=... BB_CHAVE_APLICACAO=... \
//	go test -run TestConformidadeGravada ./internal/adapters/psp/bb/
//
// A chave Pix é a de teste do BB; o pagamento usa o simulador da homologação,
// e Pagar espera a cobrança sair de ATIVA. A homologação não dispara webhook.
func TestConformidadeGravada(t *testing.T) {
	gravar := os.Getenv("VINTEM_GRAVAR_BB") != ""
	if !gravar {
		if fitas, _ := filepath.Glob(filepath.Join(dirFitas, "*.json")); len(fitas) == 0 {
			t.Skip("nenhuma fita da homologação do BB gravada ainda")
		}
	}
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		f := fita.Abrir(t, fita.Arquivo(dirFitas, t.Name()), gravar)
		cred := domain.Credenciais{ClientID: "gravado", ClientSecret: "gravado", ChaveAplicacao: "gravado"}
		if gravar {
			cred = domain.Credenciais{ClientID: os.Getenv("BB_CLIENT_ID"), ClientSecret: os.Getenv("BB_CLIENT_SECRET"), ChaveAplicacao: os.Getenv("BB_CHAVE_APLICACAO")}
			f.Dados = map[string]string{"chave": ChavePixHomologacao}
		}
		return alvoGravado(t, f, credenciais{"loja": cred}, gravar, bb.ComTransporte(f.Envolver))
	})
}

// alvoGravado é o alvo da suíte sobre uma fita: o pagamento usa o simulador
// da homologação e espera a cobrança sair de ATIVA. Gravando, espera de
// verdade e anota na fita quanto levou; na reprodução, as consultas vêm da
// fita e o relógio avança o tempo anotado.
func alvoGravado(t *testing.T, f *fita.Fita, cred credenciais, gravar bool, opcoes ...bb.Opcao) conformance.Alvo {
	t.Helper()
	relogio := fake.NewClock(f.Agora)
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "bb", ChavePix: f.Dados["chave"], Ambiente: domain.Homologacao, Ativa: true}
	p := bb.New(cred, relogio, opcoes...)
	return conformance.Alvo{
		PSP: p, Conta: conta, Agora: relogio.Now, Rodada: f.Rodada, Real: true,
		Pagar: func(t *testing.T, txid domain.Txid) {
			ctx := context.Background()
			brcode, err := brcodeDe(ctx, p, conta, txid)
			if err != nil {
				t.Fatalf("pagar %s: %v", txid, err)
			}
			if err := p.SimularPagamento(ctx, conta, brcode); err != nil {
				t.Logf("simulação recusada (cobrança removida não é paga): %v", err)
				return
			}
			inicio := time.Now()
			for tentativa := 0; ; tentativa++ {
				status, err := statusDe(ctx, p, conta, txid)
				if err != nil {
					t.Fatalf("consultando %s: %v", txid, err)
				}
				if status != domain.CobrancaAtiva {
					break
				}
				switch {
				case !gravar && tentativa > 100:
					t.Fatalf("a fita não termina com %s paga", txid)
				case gravar && time.Since(inicio) > 2*time.Minute:
					t.Fatalf("o simulador não pagou %s em 2 min", txid)
				case gravar:
					time.Sleep(3 * time.Second)
				}
			}
			chave := "pago:" + string(txid)
			if gravar {
				f.Dados[chave] = strconv.Itoa(int(time.Since(inicio).Seconds()) + 2)
			}
			segundos, _ := strconv.Atoi(f.Dados[chave])
			relogio.Avancar(time.Duration(segundos) * time.Second)
		},
	}
}

func brcodeDe(ctx context.Context, p *bb.PSP, conta domain.Conta, txid domain.Txid) (string, error) {
	if c, err := p.ConsultarCob(ctx, conta, txid); err == nil {
		return c.BRCode, nil
	}
	v, err := p.ConsultarCobv(ctx, conta, txid)
	return v.BRCode, err
}

func statusDe(ctx context.Context, p *bb.PSP, conta domain.Conta, txid domain.Txid) (domain.StatusCobranca, error) {
	c, err := p.ConsultarCob(ctx, conta, txid)
	if errors.Is(err, ports.ErrNaoEncontrado) {
		v, errV := p.ConsultarCobv(ctx, conta, txid)
		return v.Status, errV
	}
	return c.Status, err
}

// TestFitaGravaEReproduz grava a suíte contra o BB falso e reproduz sem
// servidor: o caminho das fitas (e o simulador) funciona antes de existir
// gravação de verdade, e a chave de aplicação não vai para a fita.
func TestFitaGravaEReproduz(t *testing.T) {
	dir := t.TempDir()
	arquivo := func(t *testing.T) string { return fita.Arquivo(dir, strings.SplitN(t.Name(), "/", 3)[2]) }
	srv := bbtest.Novo(t, nil, clientID, clientSecret, chaveAplicacao)
	t.Run("gravar", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), true)
			f.Dados = map[string]string{"chave": "loja@example.com"}
			cred := credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, ChaveAplicacao: chaveAplicacao}}
			return alvoGravado(t, f, cred, true, bb.ComURLs("https://producao.invalid/pix/v2", srv.URL), bb.ComRaizes(srv.Raizes), bb.ComTransporte(f.Envolver))
		})
	})
	fitas, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, arq := range fitas {
		if b, _ := os.ReadFile(arq); strings.Contains(string(b), chaveAplicacao) { //nolint:gosec // G304: fita do próprio teste
			t.Fatalf("a fita %s guardou a chave de aplicação", arq)
		}
	}
	t.Run("reproduzir", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), false)
			cred := credenciais{"loja": {ClientID: "gravado", ClientSecret: "gravado", ChaveAplicacao: "gravado"}}
			return alvoGravado(t, f, cred, false, bb.ComURLs("https://producao.invalid/pix/v2", "https://homologacao.invalid/pix/v2"), bb.ComTransporte(f.Envolver))
		})
	})
}
