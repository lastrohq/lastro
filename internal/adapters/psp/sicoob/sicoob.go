package sicoob

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

	"github.com/lastrohq/lastro/internal/adapters/psp/pixapi"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/oauth"
	"github.com/lastrohq/lastro/internal/ports"
)

// URLProducao é a base da API Pix Recebimentos do Sicoob. Não há URL de
// homologação: o sandbox do Sicoob não guarda estado (veja o doc do pacote).
const URLProducao = "https://api.sicoob.com.br/pix/api/v2"

// URLToken é o endpoint de token do Sicoob, num host separado da API.
const URLToken = "https://auth.sicoob.com.br" + caminhoToken

const caminhoToken = "/auth/realms/cooperado/protocol/openid-connect/token" //nolint:gosec // G101: caminho do endpoint, não credencial

// Escopos pedidos no token: os de cada rota que o adapter usa. A aplicação no
// portal Sicoob Developers precisa ter todos liberados.
const Escopos = "cob.read cob.write cobv.read cobv.write pix.read pix.write webhook.read webhook.write"

// PSP é o adapter do Sicoob: a API Pix da spec (pixapi) com os desvios do Sicoob.
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

// New cria o adapter. credenciais entrega o client id e o certificado de cada
// conta; o client secret, se houver, é ignorado.
func New(credenciais ports.Credenciais, clock ports.Clock, opcoes ...Opcao) *PSP {
	banco := pixapi.Banco{
		Nome:            "sicoob",
		URLs:            map[domain.Ambiente]string{domain.Producao: URLProducao},
		BuscarToken:     buscarToken,
		Traduzir:        traduzir,
		SemClientSecret: true,
		HeaderClientID:  "client_id",
		// Sem documentação pública sobre txid e id de devolução repetidos: o
		// adapter consulta antes, para cumprir o port sem depender disso.
		ConsultarAntesDeDevolver: true,
	}
	banco.ConsultarAntesDeCriar.Cob = true
	banco.ConsultarAntesDeCriar.Cobv = true
	return &PSP{API: pixapi.New(banco, credenciais, clock, opcoes...)}
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "sicoob" }

// Capabilities declara o que este adapter já implementa.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapCobv: true, ports.CapDevolucao: true, ports.CapWebhook: true}
}

// enderecoToken é o URLToken para a API de produção. Outra base (o Sicoob
// falso dos testes) serve o token no mesmo host, no mesmo caminho.
func enderecoToken(base string) string {
	if base == URLProducao {
		return URLToken
	}
	u, err := url.Parse(base)
	if err != nil {
		return URLToken
	}
	return u.Scheme + "://" + u.Host + caminhoToken
}

// buscarToken pede o token por mTLS com client_id e scope no formulário, sem
// client secret: o certificado identifica a aplicação.
func buscarToken(mtls *http.Client, base string, cred domain.Credenciais, clock ports.Clock) oauth.Buscar {
	endereco := enderecoToken(base)
	return func(ctx context.Context) (oauth.Token, error) {
		form := url.Values{
			"client_id":  {cred.ClientID},
			"grant_type": {"client_credentials"},
			"scope":      {Escopos},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endereco, strings.NewReader(form.Encode()))
		if err != nil {
			return oauth.Token{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := mtls.Do(req)
		if err != nil {
			return oauth.Token{}, fmt.Errorf("sicoob: pedindo token: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		corpo, err := io.ReadAll(io.LimitReader(resp.Body, pixapi.LimiteResposta))
		if err != nil {
			return oauth.Token{}, fmt.Errorf("sicoob: lendo token: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return oauth.Token{}, fmt.Errorf("sicoob: token recusado com status %d (confira o client id, os escopos da aplicação e se o certificado é o cadastrado nela): %w", resp.StatusCode, traduzirToken(resp.StatusCode, corpo))
		}
		return oauth.LerResposta(bytes.NewReader(corpo), clock.Now())
	}
}

// ErrSicoob é a falha do Sicoob que não corresponde a um erro conhecido do port.
var ErrSicoob = errors.New("o Sicoob recusou")

// problema junta os formatos de erro do Sicoob: o da spec do BCB (RFC 7807) e
// o do gateway, que responde antes da API.
type problema struct {
	Type            string `json:"type"`
	Title           string `json:"title"`
	Detail          string `json:"detail"`
	HTTPMessage     string `json:"httpMessage"`
	MoreInformation string `json:"moreInformation"`
}

// traduzir lê o erro do Sicoob: NaoEncontrado (ou 404) é ErrNaoEncontrado; o
// resto é ErrSicoob com o tipo e o detalhe.
func traduzir(status int, corpo []byte) error {
	var p problema
	_ = json.Unmarshal(corpo, &p) // corpo fora do formato: fica só o status
	tipo := p.Type[strings.LastIndex(p.Type, "/")+1:]
	detalhe := p.Detail
	if tipo == "" && p.HTTPMessage != "" {
		tipo, detalhe = p.HTTPMessage, p.MoreInformation
	}
	if tipo == "NaoEncontrado" || status == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ports.ErrNaoEncontrado, detalhe)
	}
	if tipo == "" {
		return fmt.Errorf("%w: status %d", ErrSicoob, status)
	}
	return fmt.Errorf("%w: status %d, %s: %s", ErrSicoob, status, tipo, detalhe)
}

// erroToken é a resposta de erro do OAuth2 (RFC 6749), que o Keycloak usa.
type erroToken struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func traduzirToken(status int, corpo []byte) error {
	var e erroToken
	if json.Unmarshal(corpo, &e) == nil && e.Error != "" {
		return fmt.Errorf("%w: %s: %s", ErrSicoob, e.Error, e.Description)
	}
	return traduzir(status, corpo)
}
