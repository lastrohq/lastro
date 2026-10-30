package bb

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

// URLs da API Pix v2 do BB por ambiente. A de produção exige mTLS; a de
// homologação, não.
const (
	URLProducao    = "https://api-pix.bb.com.br/pix/v2"
	URLHomologacao = "https://api.hm.bb.com.br/pix/v2"
)

// URLs do token, num host separado da API.
//
//nolint:gosec // G101: endereços do endpoint, não credenciais
const (
	URLTokenProducao    = "https://oauth.bb.com.br/oauth/token"
	URLTokenHomologacao = "https://oauth.hm.bb.com.br/oauth/token"
)

// Escopos pedidos no token: os de cada rota que o adapter usa. A aplicação no
// Portal Developers BB precisa ter todos liberados; sem scope, o BB recusa o
// token com invalid_scope.
const Escopos = "cob.read cob.write cobv.read cobv.write pix.read pix.write webhook.read webhook.write"

// ChaveSimulador é a chave de aplicação que o BB publica para o simulador de
// pagamento da homologação. Não é segredo.
const ChaveSimulador = "95cad3f03fd9013a9d15005056825665"

// PSP é o adapter do BB: a API Pix da spec (pixapi) com os desvios do BB.
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

// New cria o adapter. credenciais entrega o client id/secret, a chave de
// aplicação e, em produção, o certificado de cada conta.
func New(credenciais ports.Credenciais, clock ports.Clock, opcoes ...Opcao) *PSP {
	banco := pixapi.Banco{
		Nome:        "bb",
		URLs:        map[domain.Ambiente]string{domain.Producao: URLProducao, domain.Homologacao: URLHomologacao},
		BuscarToken: buscarToken,
		Traduzir:    traduzir,
		SemMTLS:     map[domain.Ambiente]bool{domain.Homologacao: true},
		Parametros:  parametros,
		// Sem documentação sobre txid e id de devolução repetidos, numa
		// homologação compartilhada: o adapter consulta antes.
		ConsultarAntesDeDevolver: true,
	}
	banco.ConsultarAntesDeCriar.Cob = true
	banco.ConsultarAntesDeCriar.Cobv = true
	return &PSP{API: pixapi.New(banco, credenciais, clock, opcoes...)}
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "bb" }

// Capabilities declara o que este adapter já implementa.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapCobv: true, ports.CapDevolucao: true, ports.CapWebhook: true}
}

// parametros põe a chave de aplicação da conta na query, com o nome do ambiente.
func parametros(amb domain.Ambiente, cred domain.Credenciais) url.Values {
	nome := "gw-app-key"
	if amb == domain.Homologacao {
		nome = "gw-dev-app-key"
	}
	return url.Values{nome: {cred.ChaveAplicacao}}
}

// ErrSemChaveAplicacao recusa a conta do BB cadastrada sem a chave de aplicação.
var ErrSemChaveAplicacao = errors.New("bb: a conta precisa da chave de aplicação do Portal Developers BB (--chave-aplicacao)")

// enderecoToken é o token do ambiente da base. Outra base (o BB falso dos
// testes) serve o token no mesmo host, em /oauth/token.
func enderecoToken(base string) string {
	switch base {
	case URLProducao:
		return URLTokenProducao
	case URLHomologacao:
		return URLTokenHomologacao
	}
	u, err := url.Parse(base)
	if err != nil {
		return URLTokenProducao
	}
	return u.Scheme + "://" + u.Host + "/oauth/token"
}

// buscarToken pede o token com client id e secret em Basic e os escopos no
// formulário.
func buscarToken(cliente *http.Client, base string, cred domain.Credenciais, clock ports.Clock) oauth.Buscar {
	endereco := enderecoToken(base)
	return func(ctx context.Context) (oauth.Token, error) {
		if cred.ChaveAplicacao == "" {
			return oauth.Token{}, ErrSemChaveAplicacao
		}
		form := url.Values{"grant_type": {"client_credentials"}, "scope": {Escopos}}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endereco, strings.NewReader(form.Encode()))
		if err != nil {
			return oauth.Token{}, err
		}
		req.SetBasicAuth(cred.ClientID, cred.ClientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := cliente.Do(req)
		if err != nil {
			return oauth.Token{}, fmt.Errorf("bb: pedindo token: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		corpo, err := io.ReadAll(io.LimitReader(resp.Body, pixapi.LimiteResposta))
		if err != nil {
			return oauth.Token{}, fmt.Errorf("bb: lendo token: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return oauth.Token{}, fmt.Errorf("bb: token recusado com status %d (confira client id, secret, os escopos da aplicação e se ela está no ambiente da conta): %w", resp.StatusCode, traduzirToken(resp.StatusCode, corpo))
		}
		return oauth.LerResposta(bytes.NewReader(corpo), clock.Now())
	}
}

// ErrBB é a falha do BB que não corresponde a um erro conhecido do port.
var ErrBB = errors.New("o BB recusou")

// mensagem é um item da lista de erros do gateway do BB, em português ou inglês.
type mensagem struct {
	Codigo   any    `json:"codigo"`
	Mensagem string `json:"mensagem"`
	Code     any    `json:"code"`
	Message  string `json:"message"`
}

func (m mensagem) texto() string {
	if m.Mensagem != "" {
		return m.Mensagem
	}
	return m.Message
}

// problema junta os formatos de erro do BB: o da spec do BCB (RFC 7807) e o
// do gateway, com uma lista de erros ou uma mensagem solta.
type problema struct {
	Type    string     `json:"type"`
	Detail  string     `json:"detail"`
	Erros   []mensagem `json:"erros"`
	Errors  []mensagem `json:"errors"`
	Message string     `json:"message"`
}

// traduzir lê o erro do BB: NaoEncontrado (ou 404) é ErrNaoEncontrado; o
// resto é ErrBB com o tipo e o detalhe.
func traduzir(status int, corpo []byte) error {
	var p problema
	_ = json.Unmarshal(corpo, &p) // corpo fora do formato: fica só o status
	tipo := p.Type[strings.LastIndex(p.Type, "/")+1:]
	detalhe := p.Detail
	if tipo == "" {
		var textos []string
		for _, m := range append(p.Erros, p.Errors...) {
			if t := m.texto(); t != "" {
				textos = append(textos, t)
			}
		}
		if p.Message != "" {
			textos = append(textos, p.Message)
		}
		detalhe = strings.Join(textos, "; ")
	}
	if tipo == "NaoEncontrado" || status == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ports.ErrNaoEncontrado, detalhe)
	}
	switch {
	case tipo != "":
		return fmt.Errorf("%w: status %d, %s: %s", ErrBB, status, tipo, detalhe)
	case detalhe != "":
		return fmt.Errorf("%w: status %d: %s", ErrBB, status, detalhe)
	}
	return fmt.Errorf("%w: status %d", ErrBB, status)
}

// erroToken é a resposta de erro do OAuth2 (RFC 6749).
type erroToken struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

func traduzirToken(status int, corpo []byte) error {
	var e erroToken
	if json.Unmarshal(corpo, &e) == nil && e.Error != "" {
		return fmt.Errorf("%w: %s: %s", ErrBB, e.Error, e.Description)
	}
	return traduzir(status, corpo)
}

// ErrSoNaHomologacao recusa a simulação de pagamento fora da homologação.
var ErrSoNaHomologacao = errors.New("bb: o simulador de pagamento só existe na homologação")

// SimularPagamento paga a cobrança pelo BR Code no simulador da homologação
// do BB, que conclui a cobrança em até uns 20 segundos. Serve para gravar a
// suíte de conformidade com pagamento; em produção é recusado antes de chamar
// o banco.
func (p *PSP) SimularPagamento(ctx context.Context, conta domain.Conta, brcode string) error {
	if conta.Ambiente != domain.Homologacao {
		return ErrSoNaHomologacao
	}
	caminho := "/testes-portal-desenvolvedor/v1/boletos-pix/pagar?gw-app-key=" + ChaveSimulador
	return p.ChamarNoHost(ctx, conta, http.MethodPost, caminho, map[string]string{"pix": brcode}, nil)
}
