package sicoob_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/sicoob"
	"github.com/vintemhq/vintem/internal/adapters/psp/sicoob/sicoobtest"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
	"github.com/vintemhq/vintem/test/conformance"
	"github.com/vintemhq/vintem/test/conformance/fita"
)

const clientID = "client-id-sicoob"

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
	psp   *sicoob.PSP
	srv   *sicoobtest.Servidor
	conta domain.Conta
}

// novo monta o adapter contra o Sicoob falso, só com client id e certificado:
// o Sicoob não tem client secret.
func novo(t *testing.T, relogio ports.Clock) cenario {
	t.Helper()
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := sicoobtest.Novo(t, par.Certificado, clientID)
	cred := credenciais{"loja": {ClientID: clientID, Certificado: par.PEM}}
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "sicoob", ChavePix: "loja@example.com", Ambiente: domain.Producao, Ativa: true}
	p := sicoob.New(cred, relogio, sicoob.ComURLs(srv.URL, "https://homologacao.invalid/pix/api/v2"), sicoob.ComRaizes(srv.Raizes))
	return cenario{psp: p, srv: srv, conta: conta}
}

// A suíte de conformidade contra o Sicoob falso.
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

func TestTokenSemSecretComEscoposEReusado(t *testing.T) {
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
	if got := c.srv.EscoposPedidos(); got != sicoob.Escopos {
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

func TestBRCodeVemDoCampoBrcode(t *testing.T) {
	c := novo(t, fake.NewClock(agora))
	ctx := context.Background()
	cob, err := c.psp.CriarCob(ctx, c.conta, domain.Cobranca{Txid: "brcode0000000000000000001", Valor: 4990, Expiracao: time.Hour, Chave: c.conta.ChavePix})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cob.BRCode, "000201") {
		t.Fatalf("BR Code: %q", cob.BRCode)
	}
	if lida, err := c.psp.ConsultarCob(ctx, c.conta, cob.Txid); err != nil || lida.BRCode != cob.BRCode {
		t.Fatalf("consulta: %q %v", lida.BRCode, err)
	}
}

func TestClientIDErradoExplicaOQueConferir(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := sicoobtest.Novo(t, par.Certificado, clientID)
	cred := credenciais{"loja": {ClientID: "errado", Certificado: par.PEM}}
	p := sicoob.New(cred, fake.NewClock(agora), sicoob.ComURLs(srv.URL, "https://x.invalid/pix/api/v2"), sicoob.ComRaizes(srv.Raizes))
	_, err := p.ConsultarCob(context.Background(), domain.Conta{ID: "loja", Ambiente: domain.Producao}, "x00000000000000000000000001")
	if err == nil || !strings.Contains(err.Error(), "invalid_client") || !strings.Contains(err.Error(), "certificado é o cadastrado") {
		t.Fatalf("err = %v", err)
	}
}

func TestContaSemCertificadoRecusada(t *testing.T) {
	cred := credenciais{"loja": {ClientID: clientID, ClientSecret: "ignorado"}}
	p := sicoob.New(cred, fake.NewClock(agora))
	_, err := p.ConsultarCob(context.Background(), domain.Conta{ID: "loja", Ambiente: domain.Producao}, "x00000000000000000000000001")
	if err == nil || !strings.Contains(err.Error(), "client id e certificado") {
		t.Fatalf("err = %v", err)
	}
}

func TestSemHomologacao(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", agora.Add(time.Hour*24))
	p := sicoob.New(credenciais{"loja": {ClientID: clientID, Certificado: par.PEM}}, fake.NewClock(agora))
	_, err := p.ConsultarCob(context.Background(), domain.Conta{ID: "loja", Ambiente: domain.Homologacao}, "x00000000000000000000000001")
	if err == nil || !strings.Contains(err.Error(), "sem URL") {
		t.Fatalf("err = %v", err)
	}
}

// O gateway do Sicoob responde antes da API num formato próprio; a API, no
// formato de problema da spec.
func TestErrosNosDoisFormatos(t *testing.T) {
	gateway := sicoob.Traduzir(401, []byte(`{"httpCode":"401","httpMessage":"Unauthorized","moreInformation":"client_id inválido"}`))
	if !errors.Is(gateway, sicoob.ErrSicoob) || !strings.Contains(gateway.Error(), "Unauthorized: client_id inválido") {
		t.Fatalf("gateway: %v", gateway)
	}
	spec := sicoob.Traduzir(400, []byte(`{"type":"https://pix.bcb.gov.br/api/v2/error/CobOperacaoInvalida","detail":"devedor inválido"}`))
	if !errors.Is(spec, sicoob.ErrSicoob) || !strings.Contains(spec.Error(), "CobOperacaoInvalida: devedor inválido") {
		t.Fatalf("spec: %v", spec)
	}
	if err := sicoob.Traduzir(404, nil); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("404: %v", err)
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

const dirFitas = "../../../../test/fixtures/sicoob"

// TestConformidadeGravada roda a suíte contra as fitas da API de produção do
// Sicoob (RN-51); o Sicoob não tem homologação com estado. Os cenários de
// pagamento são pulados: nada é pago de verdade. Para gravar, com a aplicação
// do portal Sicoob Developers:
//
//	VINTEM_GRAVAR_SICOOB=1 SICOOB_CLIENT_ID=... SICOOB_CERTIFICADO=sicoob.pem \
//	SICOOB_CHAVE_PIX=... SICOOB_WEBHOOK=https://... \
//	go test -run TestConformidadeGravada ./internal/adapters/psp/sicoob/
func TestConformidadeGravada(t *testing.T) {
	gravar := os.Getenv("VINTEM_GRAVAR_SICOOB") != ""
	if !gravar {
		if fitas, _ := filepath.Glob(filepath.Join(dirFitas, "*.json")); len(fitas) == 0 {
			t.Skip("nenhuma fita do Sicoob gravada ainda")
		}
	}
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		f := fita.Abrir(t, fita.Arquivo(dirFitas, t.Name()), gravar)
		var cred domain.Credenciais
		if gravar {
			pem, err := os.ReadFile(os.Getenv("SICOOB_CERTIFICADO")) //nolint:gosec // G703: quem grava escolhe o próprio certificado
			if err != nil {
				t.Fatal(err)
			}
			cred = domain.Credenciais{ClientID: os.Getenv("SICOOB_CLIENT_ID"), Certificado: pem}
			f.Dados = map[string]string{"chave": os.Getenv("SICOOB_CHAVE_PIX"), "webhook": os.Getenv("SICOOB_WEBHOOK")}
		} else {
			par := certificadotest.Gerar(t, "loja", f.Agora.Add(365*24*time.Hour))
			cred = domain.Credenciais{ClientID: "gravado", Certificado: par.PEM}
		}
		return alvoGravado(f, credenciais{"loja": cred}, sicoob.ComTransporte(f.Envolver))
	})
}

// alvoGravado é o alvo da suíte sobre uma fita, sem pagamento.
func alvoGravado(f *fita.Fita, cred credenciais, opcoes ...sicoob.Opcao) conformance.Alvo {
	relogio := fake.NewClock(f.Agora)
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "sicoob", ChavePix: f.Dados["chave"], Ambiente: domain.Producao, Ativa: true}
	return conformance.Alvo{
		PSP: sicoob.New(cred, relogio, opcoes...), Conta: conta, Agora: relogio.Now,
		Rodada: f.Rodada, Real: true, URLWebhook: f.Dados["webhook"],
	}
}

// TestFitaGravaEReproduz grava a suíte contra o Sicoob falso e reproduz sem
// servidor: o caminho das fitas funciona antes de existir gravação de verdade.
func TestFitaGravaEReproduz(t *testing.T) {
	dir := t.TempDir()
	arquivo := func(t *testing.T) string { return fita.Arquivo(dir, strings.SplitN(t.Name(), "/", 3)[2]) }
	par := certificadotest.Gerar(t, "loja", time.Now().Add(365*24*time.Hour))
	srv := sicoobtest.Novo(t, par.Certificado, clientID)
	t.Run("gravar", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), true)
			f.Dados = map[string]string{"chave": "loja@example.com"}
			cred := credenciais{"loja": {ClientID: clientID, Certificado: par.PEM}}
			return alvoGravado(f, cred, sicoob.ComURLs(srv.URL, "https://homologacao.invalid/pix/api/v2"), sicoob.ComRaizes(srv.Raizes), sicoob.ComTransporte(f.Envolver))
		})
	})
	t.Run("reproduzir", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), false)
			p := certificadotest.Gerar(t, "loja", f.Agora.Add(365*24*time.Hour))
			cred := credenciais{"loja": {ClientID: "gravado", Certificado: p.PEM}}
			return alvoGravado(f, cred, sicoob.ComURLs("https://producao.invalid/pix/api/v2", "https://homologacao.invalid/pix/api/v2"), sicoob.ComTransporte(f.Envolver))
		})
	})
}
