// Package pixapi é o cliente da API Pix do Banco Central, compartilhado pelos
// adapters de banco (RN-51). Um banco é uma configuração (Banco): URLs, como
// pedir o token, como ler os erros e quais desvios da spec tratar. O que é só
// dele, como um formato de webhook próprio, fica no pacote do banco, que
// embute *API e sobrescreve o método.
package pixapi

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado"
	"github.com/vintemhq/vintem/internal/platform/oauth"
	"github.com/vintemhq/vintem/internal/ports"
)

// Banco descreve o que muda de um banco para outro.
type Banco struct {
	// Nome prefixa os erros ("efi: ...").
	Nome string
	// URLs da API Pix por ambiente, sem barra no fim.
	URLs map[domain.Ambiente]string
	// Prefixo vem antes de /cob, /cobv, /pix e /webhook (o Efí usa "/v2";
	// quem já tem a versão na URL base usa "").
	Prefixo string
	// BuscarToken monta a busca do token OAuth2 da conta, pelo cliente mTLS.
	BuscarToken func(mtls *http.Client, base string, cred domain.Credenciais, clock ports.Clock) oauth.Buscar
	// Traduzir lê a resposta de erro do banco e devolve o erro do port
	// (ErrJaExiste, ErrNaoEncontrado...). Use ErrDevolucaoDuplicada para id de
	// devolução repetido.
	Traduzir func(status int, corpo []byte) error
	// ConsultarAntesDeCriar faz CriarCob/CriarCobv consultarem o txid antes do
	// PUT, para o banco que sobrescreve ou não sinaliza txid repetido.
	ConsultarAntesDeCriar struct{ Cob, Cobv bool }
	// ConsultarAntesDeDevolver faz o Devolver consultar o id antes do PUT,
	// para o banco que não sinaliza id de devolução repetido: mesma devolução
	// (mesmo valor) é devolvida de novo; valor diferente é ErrJaExiste.
	ConsultarAntesDeDevolver bool
	// HeadersWebhook vão no PUT do webhook (ex.: o skip-mTLS do Efí).
	HeadersWebhook map[string]string
	// SemClientSecret é para o banco que identifica a aplicação pelo
	// certificado e só pede o client id (o Sicoob): a conta não precisa de
	// client secret.
	SemClientSecret bool
	// HeaderClientID, se não vazio, é o header que leva o client id em toda
	// chamada à API Pix (o Sicoob usa "client_id").
	HeaderClientID string
	// SemMTLS lista os ambientes em que o banco não pede certificado (a
	// homologação do BB): a conta pode vir sem ele, e a chamada vai em TLS
	// comum. Com certificado, ele é usado do mesmo jeito.
	SemMTLS map[domain.Ambiente]bool
	// Parametros, se não nil, entram na query de toda chamada à API Pix (a
	// chave de aplicação do BB). O token não leva.
	Parametros func(domain.Ambiente, domain.Credenciais) url.Values
}

// API é o cliente da API Pix de um banco. Guarda um cliente HTTP (mTLS +
// OAuth2) por conta, montado de novo quando as credenciais mudam.
type API struct {
	cfg         Banco
	credenciais ports.Credenciais
	clock       ports.Clock
	raizes      *x509.CertPool
	timeout     time.Duration
	tokens      *oauth.Cache
	envolver    func(http.RoundTripper) http.RoundTripper

	mu       sync.Mutex
	clientes map[string]*cliente
}

// Opcao configura a API.
type Opcao func(*API)

// ComURLs troca as URLs dos ambientes (nos testes, um servidor falso).
func ComURLs(producao, homologacao string) Opcao {
	return func(a *API) {
		a.cfg.URLs = map[domain.Ambiente]string{domain.Producao: producao, domain.Homologacao: homologacao}
	}
}

// ComRaizes define em quem confiar no TLS do banco; nil usa as raízes do sistema.
func ComRaizes(raizes *x509.CertPool) Opcao { return func(a *API) { a.raizes = raizes } }

// ComTimeout limita cada chamada ao banco (padrão 30 s).
func ComTimeout(d time.Duration) Opcao { return func(a *API) { a.timeout = d } }

// ComTransporte envolve o transporte mTLS de cada conta, token incluído (nos
// testes de conformidade, a fita que grava e reproduz o banco de verdade).
func ComTransporte(envolver func(http.RoundTripper) http.RoundTripper) Opcao {
	return func(a *API) { a.envolver = envolver }
}

// New cria a API do banco. credenciais entrega o client id/secret e o
// certificado de cada conta.
func New(cfg Banco, credenciais ports.Credenciais, clock ports.Clock, opcoes ...Opcao) *API {
	a := &API{
		cfg:         cfg,
		credenciais: credenciais,
		clock:       clock,
		timeout:     30 * time.Second,
		tokens:      oauth.NovoCache(clock),
		clientes:    map[string]*cliente{},
	}
	for _, o := range opcoes {
		o(a)
	}
	return a
}

// InterpretarWebhook lê o aviso no formato do callback da spec do BCB.
func (a *API) InterpretarWebhook(conta domain.Conta, corpo []byte) ([]domain.PixRecebido, error) {
	return psp.WebhookPixBCB(conta, corpo)
}

// cliente é a ligação com o banco de uma conta, com as credenciais de quando foi montado.
type cliente struct {
	http       *http.Client
	base       string
	clientID   string
	parametros url.Values
	versao     [32]byte // resumo das credenciais e do ambiente; mudou, monta de novo
}

// clienteDa devolve o cliente da conta, montando de novo se as credenciais ou o
// ambiente mudaram desde a última vez.
func (a *API) clienteDa(ctx context.Context, conta domain.Conta) (*cliente, error) {
	nome := a.cfg.Nome
	cred, err := a.credenciais.Credenciais(ctx, conta.ID)
	if err != nil {
		return nil, fmt.Errorf("%s: credenciais da conta %s: %w", nome, conta.ID, err)
	}
	base, ok := a.cfg.URLs[conta.Ambiente]
	if !ok {
		return nil, fmt.Errorf("%s: ambiente %q sem URL", nome, conta.Ambiente)
	}
	versao := resumo(cred, base)

	a.mu.Lock()
	defer a.mu.Unlock()
	if c := a.clientes[conta.ID]; c != nil && c.versao == versao {
		return c, nil
	}
	semMTLS := a.cfg.SemMTLS[conta.Ambiente]
	exigidos := "client id, client secret e certificado"
	switch {
	case a.cfg.SemClientSecret:
		exigidos = "client id e certificado"
	case semMTLS:
		exigidos = "client id e client secret"
	}
	if cred.ClientID == "" || (cred.ClientSecret == "" && !a.cfg.SemClientSecret) || (len(cred.Certificado) == 0 && !semMTLS) {
		return nil, fmt.Errorf("%s: a conta %s precisa de %s", nome, conta.ID, exigidos)
	}
	var certificados []tls.Certificate
	if len(cred.Certificado) > 0 {
		par, err := certificado.TLS(cred.Certificado, cred.SenhaCertificado)
		if err != nil {
			return nil, fmt.Errorf("%s: certificado da conta %s: %w", nome, conta.ID, err)
		}
		certificados = []tls.Certificate{par}
	}
	var mtls http.RoundTripper = &http.Transport{
		TLSClientConfig:     &tls.Config{Certificates: certificados, RootCAs: a.raizes, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if a.envolver != nil {
		mtls = a.envolver(mtls)
	}
	// O token é guardado por versão das credenciais: trocar o secret não reusa o token velho.
	chaveToken := fmt.Sprintf("%s/%x", conta.ID, versao[:8])
	buscar := a.cfg.BuscarToken(&http.Client{Transport: mtls, Timeout: a.timeout}, base, cred, a.clock)
	c := &cliente{
		http:     &http.Client{Transport: &oauth.Transport{Base: mtls, Cache: a.tokens, Chave: chaveToken, Buscar: buscar}, Timeout: a.timeout},
		base:     base,
		clientID: cred.ClientID,
		versao:   versao,
	}
	if a.cfg.Parametros != nil {
		c.parametros = a.cfg.Parametros(conta.Ambiente, cred)
	}
	if antigo := a.clientes[conta.ID]; antigo != nil {
		antigo.http.CloseIdleConnections()
	}
	a.clientes[conta.ID] = c
	return c, nil
}

func resumo(cred domain.Credenciais, base string) [32]byte {
	h := sha256.New()
	for _, parte := range [][]byte{[]byte(base), []byte(cred.ClientID), []byte(cred.ClientSecret), cred.Certificado, []byte(cred.SenhaCertificado), []byte(cred.ChaveAplicacao)} {
		_, _ = fmt.Fprintf(h, "%d:", len(parte))
		_, _ = h.Write(parte)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
