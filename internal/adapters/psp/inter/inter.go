package inter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/vintemhq/vintem/internal/adapters/psp/pixapi"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/oauth"
	"github.com/vintemhq/vintem/internal/ports"
)

// URLs da API Pix do Inter por ambiente (servers do swagger-api-pix.yaml).
// O token fica no mesmo host, em /oauth/v2/token.
const (
	URLProducao    = "https://cdpj.partners.bancointer.com.br/pix/v2"
	URLHomologacao = "https://cdpj-sandbox.partners.uatinter.co/pix/v2"
)

// Escopos pedidos no token: os de cada rota que o adapter usa. A aplicação no
// Internet Banking PJ precisa ter todos marcados.
const Escopos = "cob.read cob.write cobv.read cobv.write pix.read pix.write webhook.read webhook.write"

// PSP é o adapter do Inter: a API Pix da spec (pixapi) com os desvios do Inter.
type PSP struct {
	*pixapi.API
}

var _ ports.PSP = (*PSP)(nil)

// Opcao configura o adapter.
type Opcao = pixapi.Opcao

// Opções do adapter (veja pixapi): URLs, raízes do TLS, timeout e transporte.
var (
	ComURLs       = pixapi.ComURLs
	ComRaizes     = pixapi.ComRaizes
	ComTimeout    = pixapi.ComTimeout
	ComTransporte = pixapi.ComTransporte
)

// New cria o adapter. credenciais entrega o client id/secret e o certificado
// (o .crt e o .key do Inter juntos num PEM) de cada conta.
func New(credenciais ports.Credenciais, clock ports.Clock, opcoes ...Opcao) *PSP {
	banco := pixapi.Banco{
		Nome:        "inter",
		URLs:        map[domain.Ambiente]string{domain.Producao: URLProducao, domain.Homologacao: URLHomologacao},
		BuscarToken: buscarToken,
		Traduzir:    traduzir,
		// A documentação não diz o que acontece com txid nem com id de
		// devolução repetidos (o PUT /cob só lista 400): o adapter consulta
		// antes, para cumprir o port sem depender disso.
		ConsultarAntesDeDevolver: true,
	}
	banco.ConsultarAntesDeCriar.Cob = true
	banco.ConsultarAntesDeCriar.Cobv = true
	return &PSP{API: pixapi.New(banco, credenciais, clock, opcoes...)}
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "inter" }

// Capabilities declara o que este adapter já implementa.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapCobv: true, ports.CapDevolucao: true, ports.CapWebhook: true}
}

// buscarToken pede o token em POST /oauth/v2/token: client_id e client_secret
// no corpo do formulário (não em Basic), com os escopos, por mTLS. O Inter
// limita a 5 tokens por minuto; o oauth.Cache reusa o token por uma hora.
func buscarToken(mtls *http.Client, base string, cred domain.Credenciais, clock ports.Clock) oauth.Buscar {
	endereco := strings.TrimSuffix(base, "/pix/v2") + "/oauth/v2/token"
	return func(ctx context.Context) (oauth.Token, error) {
		form := url.Values{
			"client_id":     {cred.ClientID},
			"client_secret": {cred.ClientSecret},
			"grant_type":    {"client_credentials"},
			"scope":         {Escopos},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endereco, strings.NewReader(form.Encode()))
		if err != nil {
			return oauth.Token{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := mtls.Do(req)
		if err != nil {
			return oauth.Token{}, fmt.Errorf("inter: pedindo token: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		corpo, err := io.ReadAll(io.LimitReader(resp.Body, pixapi.LimiteResposta))
		if err != nil {
			return oauth.Token{}, fmt.Errorf("inter: lendo token: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return oauth.Token{}, fmt.Errorf("inter: token recusado com status %d (confira client id, secret, escopos da aplicação e se o certificado é do mesmo ambiente): %w", resp.StatusCode, traduzir(resp.StatusCode, corpo))
		}
		return oauth.LerResposta(bytes.NewReader(corpo), clock.Now())
	}
}

// ErrInter é a falha do Inter que não corresponde a um erro conhecido do port.
var ErrInter = errors.New("o Inter recusou")

// problema é o erro no formato da spec do BCB (RFC 7807), que o Inter segue.
type problema struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// traduzir lê o problema do Inter: NaoEncontrado (ou 404) é ErrNaoEncontrado;
// o resto é ErrInter com o tipo e o detalhe.
func traduzir(status int, corpo []byte) error {
	var p problema
	_ = json.Unmarshal(corpo, &p) // corpo fora do formato: fica só o status
	tipo := p.Type[strings.LastIndex(p.Type, "/")+1:]
	if tipo == "NaoEncontrado" || status == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ports.ErrNaoEncontrado, p.Detail)
	}
	if tipo == "" {
		return fmt.Errorf("%w: status %d", ErrInter, status)
	}
	return fmt.Errorf("%w: status %d, %s: %s", ErrInter, status, tipo, p.Detail)
}

// ErrSoNaHomologacao recusa a simulação de pagamento fora da homologação.
var ErrSoNaHomologacao = errors.New("inter: simulação de pagamento só existe no sandbox (homologação)")

// SimularPagamento paga a cobrança pelo BR Code no sandbox do Inter (POST
// /sandbox/cob/pagamento). Serve para gravar a suíte de conformidade com
// pagamento; em produção é recusado antes de chamar o banco.
func (p *PSP) SimularPagamento(ctx context.Context, conta domain.Conta, brcode string, v domain.Money) error {
	if conta.Ambiente != domain.Homologacao {
		return ErrSoNaHomologacao
	}
	// Desvio: aqui o valor é número em reais, não a string "49.90" da spec.
	corpo := map[string]any{"qrCode": brcode, "valor": json.Number(v.String())}
	return p.Chamar(ctx, conta, http.MethodPost, "/sandbox/cob/pagamento", corpo, nil)
}
