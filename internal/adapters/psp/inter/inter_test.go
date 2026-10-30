package inter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/inter"
	"github.com/vintemhq/vintem/internal/adapters/psp/inter/intertest"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
	"github.com/vintemhq/vintem/test/conformance"
	"github.com/vintemhq/vintem/test/conformance/fita"
)

const (
	clientID     = "client-id-inter"
	clientSecret = "client-secret-que-nao-pode-vazar"
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
	psp   *inter.PSP
	srv   *intertest.Servidor
	conta domain.Conta
}

// novo monta o adapter contra o Inter falso, com o certificado em PEM (o
// .crt e o .key do Inter num arquivo só, como o cadastro da conta pede).
func novo(t *testing.T, relogio ports.Clock) cenario {
	t.Helper()
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := intertest.Novo(t, par.Certificado, clientID, clientSecret)
	cred := credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, Certificado: par.PEM}}
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "inter", ChavePix: "loja@example.com", Ambiente: domain.Homologacao, Ativa: true}
	p := inter.New(cred, relogio, inter.ComURLs("https://producao.invalid/pix/v2", srv.URL), inter.ComRaizes(srv.Raizes))
	return cenario{psp: p, srv: srv, conta: conta}
}

// A suíte de conformidade contra o Inter falso, escrito pelo swagger oficial.
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

func TestTokenPorFormularioComEscoposEReusado(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	ctx := context.Background()
	for i := range 3 {
		if _, err := c.psp.ConsultarCob(ctx, c.conta, domain.Txid("inexistente000000000000000"+string(rune('a'+i)))); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("consulta %d: %v", i, err)
		}
	}
	// O Inter limita a 5 tokens por minuto: três chamadas, um token.
	if n := c.srv.TokensEmitidos(); n != 1 {
		t.Fatalf("%d tokens emitidos, quer 1", n)
	}
	if got := c.srv.EscoposPedidos(); got != inter.Escopos {
		t.Fatalf("escopos pedidos: %q", got)
	}
	c.srv.RevogarTokens()
	if _, err := c.psp.ConsultarCob(ctx, c.conta, "inexistente00000000000000zz"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("depois de revogar: %v", err)
	}
	if n := c.srv.TokensEmitidos(); n != 2 {
		t.Fatalf("com token revogado (401), pede outro e tenta de novo: %d tokens", n)
	}
}

func TestCredencialErradaExplicaOQueConferir(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := intertest.Novo(t, par.Certificado, clientID, clientSecret)
	cred := credenciais{"loja": {ClientID: clientID, ClientSecret: "errado", Certificado: par.PEM}}
	p := inter.New(cred, fake.NewClock(agora), inter.ComURLs("https://x.invalid/pix/v2", srv.URL), inter.ComRaizes(srv.Raizes))
	_, err := p.ConsultarCob(context.Background(), domain.Conta{ID: "loja", Ambiente: domain.Homologacao}, "x00000000000000000000000001")
	if err == nil || !strings.Contains(err.Error(), "escopos da aplicação") {
		t.Fatalf("err = %v", err)
	}
}

func TestSimularPagamentoNoSandbox(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	ctx := context.Background()
	cob, err := c.psp.CriarCob(ctx, c.conta, domain.Cobranca{Txid: "simulacao00000000000000001", Valor: 4990, Expiracao: time.Hour, Chave: c.conta.ChavePix})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.psp.SimularPagamento(ctx, c.conta, cob.BRCode, cob.Valor); err != nil {
		t.Fatal(err)
	}
	if lida, err := c.psp.ConsultarCob(ctx, c.conta, cob.Txid); err != nil || lida.Status != domain.CobrancaConcluida {
		t.Fatalf("depois de simular: %s %v", lida.Status, err)
	}
	producao := c.conta
	producao.Ambiente = domain.Producao
	if err := c.psp.SimularPagamento(ctx, producao, cob.BRCode, cob.Valor); !errors.Is(err, inter.ErrSoNaHomologacao) {
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

const dirFitas = "../../../../test/fixtures/inter"

// TestConformidadeGravada roda a suíte contra as fitas do sandbox do Inter
// (RN-51). Para gravar, com uma integração de sandbox do Inter Empresas:
//
//	VINTEM_GRAVAR_INTER=1 INTER_CLIENT_ID=... INTER_CLIENT_SECRET=... \
//	INTER_CERTIFICADO=inter.pem INTER_CHAVE_PIX=... \
//	go test -run TestConformidadeGravada ./internal/adapters/psp/inter/
//
// inter.pem é o .crt e o .key da integração num arquivo só. O pagamento usa a
// simulação do sandbox (POST /sandbox/cob/pagamento).
func TestConformidadeGravada(t *testing.T) {
	gravar := os.Getenv("VINTEM_GRAVAR_INTER") != ""
	if !gravar {
		if fitas, _ := filepath.Glob(filepath.Join(dirFitas, "*.json")); len(fitas) == 0 {
			t.Skip("nenhuma fita do sandbox do Inter gravada ainda")
		}
	}
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		f := fita.Abrir(t, fita.Arquivo(dirFitas, t.Name()), gravar)
		var cred domain.Credenciais
		if gravar {
			pem, err := os.ReadFile(os.Getenv("INTER_CERTIFICADO")) //nolint:gosec // G703: quem grava escolhe o próprio certificado
			if err != nil {
				t.Fatal(err)
			}
			cred = domain.Credenciais{ClientID: os.Getenv("INTER_CLIENT_ID"), ClientSecret: os.Getenv("INTER_CLIENT_SECRET"), Certificado: pem}
			f.Dados = map[string]string{"chave": os.Getenv("INTER_CHAVE_PIX")}
		} else {
			par := certificadotest.Gerar(t, "loja", f.Agora.Add(365*24*time.Hour))
			cred = domain.Credenciais{ClientID: "gravado", ClientSecret: "gravado", Certificado: par.PEM}
		}
		return alvoGravado(f, credenciais{"loja": cred}, f.Dados["chave"], inter.ComTransporte(f.Envolver))
	})
}

// alvoGravado é o alvo da suíte sobre uma fita: o pagamento usa a simulação
// do sandbox do Inter, pelo BR Code da cobrança.
func alvoGravado(f *fita.Fita, cred credenciais, chave string, opcoes ...inter.Opcao) conformance.Alvo {
	relogio := fake.NewClock(f.Agora)
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "inter", ChavePix: chave, Ambiente: domain.Homologacao, Ativa: true}
	p := inter.New(cred, relogio, opcoes...)
	{
		return conformance.Alvo{
			PSP: p, Conta: conta, Agora: relogio.Now, Rodada: f.Rodada, Real: true, URLWebhook: f.Dados["webhook"],
			Pagar: func(t *testing.T, txid domain.Txid) {
				ctx := context.Background()
				var brcode string
				var valor domain.Money
				if c, err := p.ConsultarCob(ctx, conta, txid); err == nil {
					brcode, valor = c.BRCode, c.Valor
				} else if v, err := p.ConsultarCobv(ctx, conta, txid); err == nil {
					brcode, valor = v.BRCode, v.Valor
				} else {
					t.Fatalf("pagar %s: %v", txid, err)
				}
				if err := p.SimularPagamento(ctx, conta, brcode, valor); err != nil {
					t.Logf("simulação recusada (cobrança removida não é paga): %v", err)
				}
				relogio.Avancar(2 * time.Second)
			},
		}
	}
}

// TestFitaGravaEReproduz grava a suíte contra o Inter falso e reproduz sem
// servidor: o caminho das fitas (e o pagamento simulado) funciona antes de
// existir gravação de verdade.
func TestFitaGravaEReproduz(t *testing.T) {
	dir := t.TempDir()
	arquivo := func(t *testing.T) string { return fita.Arquivo(dir, strings.SplitN(t.Name(), "/", 3)[2]) }
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := intertest.Novo(t, par.Certificado, clientID, clientSecret)
	t.Run("gravar", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), true)
			f.Dados = map[string]string{"chave": "loja@example.com"}
			cred := credenciais{"loja": {ClientID: clientID, ClientSecret: clientSecret, Certificado: par.PEM}}
			return alvoGravado(f, cred, "loja@example.com", inter.ComURLs("https://producao.invalid/pix/v2", srv.URL), inter.ComRaizes(srv.Raizes), inter.ComTransporte(f.Envolver))
		})
	})
	t.Run("reproduzir", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), false)
			p := certificadotest.Gerar(t, "loja", f.Agora.Add(365*24*time.Hour))
			cred := credenciais{"loja": {ClientID: "gravado", ClientSecret: "gravado", Certificado: p.PEM}}
			return alvoGravado(f, cred, f.Dados["chave"], inter.ComURLs("https://producao.invalid/pix/v2", "https://homologacao.invalid/pix/v2"), inter.ComTransporte(f.Envolver))
		})
	})
}
