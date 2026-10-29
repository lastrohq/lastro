package efi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp/efi"
	"github.com/lastrohq/lastro/internal/adapters/psp/efi/efitest"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/certificado/certificadotest"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
	"github.com/lastrohq/lastro/pkg/brcode"
)

const (
	clientID     = "Client_Id_teste"
	clientSecret = "Client_Secret_que_nao_pode_vazar"
)

var agora = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

// credenciais é o ports.Credenciais em memória, trocável no meio do teste.
type credenciais struct {
	mu    sync.Mutex
	porID map[string]domain.Credenciais
}

func (c *credenciais) Credenciais(_ context.Context, id string) (domain.Credenciais, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cred, ok := c.porID[id]
	if !ok {
		return domain.Credenciais{}, ports.ErrNaoEncontrado
	}
	return cred, nil
}

func (c *credenciais) trocar(id string, cred domain.Credenciais) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.porID[id] = cred
}

type cenario struct {
	psp   *efi.PSP
	efi   *efitest.Servidor
	cred  *credenciais
	conta domain.Conta
	par   certificadotest.Par
}

func novo(t *testing.T) cenario {
	t.Helper()
	par := certificadotest.Gerar(t, "loja", agora.Add(365*24*time.Hour))
	srv := efitest.Novo(t, par.Certificado, clientID, clientSecret)
	cred := &credenciais{porID: map[string]domain.Credenciais{
		"loja": {ClientID: clientID, ClientSecret: clientSecret, Certificado: par.P12, SenhaCertificado: certificadotest.Senha},
	}}
	conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "efi", ChavePix: "loja@example.com", Ambiente: domain.Homologacao, Ativa: true}
	p := efi.New(cred, fake.NewClock(agora), efi.ComURLs("https://producao.invalid", srv.URL), efi.ComRaizes(srv.Raizes))
	return cenario{psp: p, efi: srv, cred: cred, conta: conta, par: par}
}

func cob(txid string) domain.Cobranca {
	return domain.Cobranca{
		Txid: domain.Txid(txid), Chave: "loja@example.com", Valor: 1050, Expiracao: time.Hour,
		Devedor: &domain.Devedor{Documento: "12345678909", Nome: "Francisco da Silva"}, SolicitacaoPagador: "Pedido 42",
	}
}

func TestCriarEConsultarCob(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	criada, err := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000001"))
	if err != nil {
		t.Fatal(err)
	}
	if criada.Status != domain.CobrancaAtiva || criada.Location == "" || criada.Revisao != 0 || criada.ContaID != "loja" {
		t.Fatalf("criada = %+v", criada)
	}
	p, err := brcode.Decode(criada.BRCode)
	if err != nil || p.URL != criada.Location {
		t.Fatalf("BR Code %q não aponta para a location %q: %v", criada.BRCode, criada.Location, err)
	}

	// O GET do Efí devolve expiracao como string: tem que chegar como 1 h.
	lida, err := c.psp.ConsultarCob(ctx, c.conta, "efi00000000000000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if lida.Valor != 1050 || lida.Expiracao != time.Hour || lida.Devedor == nil || lida.Devedor.Documento != "12345678909" ||
		lida.SolicitacaoPagador != "Pedido 42" || lida.Chave != "loja@example.com" || lida.CriadaEm.IsZero() {
		t.Fatalf("lida = %+v (devedor %+v)", lida, lida.Devedor)
	}
}

func TestErrosDoEfiViramErrosDoPort(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	if _, err := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000002")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000002")); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("txid repetido: err = %v; quer ErrJaExiste", err)
	}
	if _, err := c.psp.ConsultarCob(ctx, c.conta, "efiinexistente00000000000001"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("inexistente: err = %v; quer ErrNaoEncontrado", err)
	}
	c.efi.Falhar(http.StatusInternalServerError)
	_, err := c.psp.ConsultarCob(ctx, c.conta, "efi00000000000000000000002")
	if !errors.Is(err, efi.ErrEfi) || errors.Is(err, ports.ErrNaoEncontrado) || !strings.Contains(err.Error(), "500") {
		t.Fatalf("500: err = %v; quer ErrEfi com o status", err)
	}
}

func TestRevisarERemoverCob(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	original, _ := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000003"))

	revisada := original
	revisada.Valor = 2000
	got, err := c.psp.RevisarCob(ctx, c.conta, revisada)
	if err != nil || got.Valor != 2000 || got.Revisao != 1 || got.Status != domain.CobrancaAtiva {
		t.Fatalf("revisar: %+v, %v", got, err)
	}

	removida := got
	removida.Status = domain.CobrancaRemovidaPeloUsuarioRecebedor
	if got, err = c.psp.RevisarCob(ctx, c.conta, removida); err != nil || got.Status != domain.CobrancaRemovidaPeloUsuarioRecebedor {
		t.Fatalf("remover: %+v, %v", got, err)
	}
	if _, err := c.psp.RevisarCob(ctx, c.conta, revisada); !errors.Is(err, domain.ErrTransicaoInvalida) {
		t.Fatalf("revisar removida: err = %v; quer ErrTransicaoInvalida", err)
	}
}

func TestTokenEReutilizadoERenovadoQuandoORevogam(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	for i := range 3 {
		if _, err := c.psp.ConsultarCob(ctx, c.conta, "efiinexistente00000000000001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("chamada %d: %v", i, err)
		}
	}
	if n := c.efi.TokensEmitidos(); n != 1 {
		t.Fatalf("tokens = %d em 3 chamadas; quer 1", n)
	}

	c.efi.RevogarTokens() // o Efí derruba o token antes de expirar
	if _, err := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000004")); err != nil {
		t.Fatalf("depois de revogar: %v", err)
	}
	if n := c.efi.TokensEmitidos(); n != 2 {
		t.Fatalf("tokens = %d; quer 2 (um novo depois do 401)", n)
	}
}

func TestAmbienteDaContaEscolheAURL(t *testing.T) {
	c := novo(t)
	prod := c.conta
	prod.Ambiente = domain.Producao // aponta para https://producao.invalid
	if _, err := c.psp.ConsultarCob(context.Background(), prod, "efi00000000000000000000001"); err == nil || !strings.Contains(err.Error(), "producao.invalid") {
		t.Fatalf("conta de produção não foi para a URL de produção: %v", err)
	}
	if len(c.efi.Chamadas()) != 0 {
		t.Fatalf("conta de produção bateu na homologação: %v", c.efi.Chamadas())
	}
}

func TestCredenciaisErradas(t *testing.T) {
	t.Run("secret errado", func(t *testing.T) {
		c := novo(t)
		cred, _ := c.cred.Credenciais(context.Background(), "loja")
		cred.ClientSecret = "secret-errado-que-nao-pode-vazar"
		c.cred.trocar("loja", cred)
		_, err := c.psp.ConsultarCob(context.Background(), c.conta, "efi00000000000000000000001")
		if err == nil || !strings.Contains(err.Error(), "token recusado") {
			t.Fatalf("err = %v; quer token recusado", err)
		}
		if strings.Contains(err.Error(), "secret-errado-que-nao-pode-vazar") || strings.Contains(err.Error(), clientID) {
			t.Fatalf("o erro mostrou credencial: %v", err)
		}
	})
	t.Run("certificado que o Efí não reconhece", func(t *testing.T) {
		c := novo(t)
		outro := certificadotest.Gerar(t, "intruso", agora.Add(time.Hour*24))
		c.cred.trocar("loja", domain.Credenciais{ClientID: clientID, ClientSecret: clientSecret, Certificado: outro.PEM})
		if _, err := c.psp.ConsultarCob(context.Background(), c.conta, "efi00000000000000000000001"); err == nil {
			t.Fatal("o Efí aceitou um certificado de cliente que não emitiu")
		}
		if n := c.efi.TokensEmitidos(); n != 0 {
			t.Fatalf("token emitido sem mTLS válido: %d", n)
		}
	})
	t.Run("conta sem certificado", func(t *testing.T) {
		c := novo(t)
		c.cred.trocar("loja", domain.Credenciais{ClientID: clientID, ClientSecret: clientSecret})
		_, err := c.psp.ConsultarCob(context.Background(), c.conta, "efi00000000000000000000001")
		if err == nil || !strings.Contains(err.Error(), "precisa de client id, client secret e certificado") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTrocarCredenciaisMontaOutroCliente(t *testing.T) {
	c := novo(t)
	ctx := context.Background()
	if _, err := c.psp.CriarCob(ctx, c.conta, cob("efi00000000000000000000005")); err != nil {
		t.Fatal(err)
	}
	// O operador recadastra a conta com o mesmo certificado em PEM: credenciais novas, token novo.
	c.cred.trocar("loja", domain.Credenciais{ClientID: clientID, ClientSecret: clientSecret, Certificado: c.par.PEM})
	if _, err := c.psp.ConsultarCob(ctx, c.conta, "efi00000000000000000000005"); err != nil {
		t.Fatal(err)
	}
	if n := c.efi.TokensEmitidos(); n != 2 {
		t.Fatalf("tokens = %d; quer 2 (credenciais trocadas não reusam o token velho)", n)
	}
}

func TestDeclaraAsCapacidadesImplementadas(t *testing.T) {
	c := novo(t)
	for _, capacidade := range []ports.Capacidade{ports.CapCob, ports.CapCobv, ports.CapDevolucao, ports.CapWebhook} {
		if err := c.psp.Capabilities().Suporta(capacidade); err != nil {
			t.Errorf("%s: %v", capacidade, err)
		}
	}
	// Pix Automático é a F6.
	if err := c.psp.Capabilities().Suporta(ports.CapRec); !errors.Is(err, ports.ErrNaoSuportado) {
		t.Errorf("declara rec antes da F6: %v", err)
	}
}
