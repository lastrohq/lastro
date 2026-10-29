package efi

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/certificado"
	"github.com/lastrohq/lastro/internal/platform/oauth"
	"github.com/lastrohq/lastro/internal/ports"
)

// URLs da API Pix do Efí por ambiente.
const (
	URLProducao    = "https://pix.api.efipay.com.br"
	URLHomologacao = "https://pix-h.api.efipay.com.br"
)

// PSP é o adapter do Efí. Guarda um cliente HTTP (mTLS + OAuth2) por conta.
type PSP struct {
	credenciais ports.Credenciais
	clock       ports.Clock
	urls        map[domain.Ambiente]string
	raizes      *x509.CertPool
	timeout     time.Duration
	tokens      *oauth.Cache

	mu       sync.Mutex
	clientes map[string]*cliente
}

var _ ports.PSP = (*PSP)(nil)

// Opcao configura o adapter.
type Opcao func(*PSP)

// ComURLs troca as URLs dos ambientes (nos testes, um servidor falso).
func ComURLs(producao, homologacao string) Opcao {
	return func(p *PSP) {
		p.urls = map[domain.Ambiente]string{domain.Producao: producao, domain.Homologacao: homologacao}
	}
}

// ComRaizes define em quem confiar no TLS do Efí; nil usa as raízes do sistema.
func ComRaizes(raizes *x509.CertPool) Opcao { return func(p *PSP) { p.raizes = raizes } }

// ComTimeout limita cada chamada ao Efí (padrão 30 s).
func ComTimeout(d time.Duration) Opcao { return func(p *PSP) { p.timeout = d } }

// New cria o adapter. credenciais entrega o client id/secret e o certificado
// de cada conta.
func New(credenciais ports.Credenciais, clock ports.Clock, opcoes ...Opcao) *PSP {
	p := &PSP{
		credenciais: credenciais,
		clock:       clock,
		urls:        map[domain.Ambiente]string{domain.Producao: URLProducao, domain.Homologacao: URLHomologacao},
		timeout:     30 * time.Second,
		tokens:      oauth.NovoCache(clock),
		clientes:    map[string]*cliente{},
	}
	for _, o := range opcoes {
		o(p)
	}
	return p
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "efi" }

// Capabilities declara o que este adapter já implementa.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapCobv: true, ports.CapDevolucao: true}
}

// cliente é a ligação com o Efí de uma conta, com as credenciais de quando foi montado.
type cliente struct {
	http   *http.Client
	base   string
	versao [32]byte // resumo das credenciais e do ambiente; mudou, monta de novo
}

// clienteDa devolve o cliente da conta, montando de novo se as credenciais ou o
// ambiente mudaram desde a última vez.
func (p *PSP) clienteDa(ctx context.Context, conta domain.Conta) (*cliente, error) {
	cred, err := p.credenciais.Credenciais(ctx, conta.ID)
	if err != nil {
		return nil, fmt.Errorf("efi: credenciais da conta %s: %w", conta.ID, err)
	}
	base, ok := p.urls[conta.Ambiente]
	if !ok {
		return nil, fmt.Errorf("efi: ambiente %q sem URL", conta.Ambiente)
	}
	versao := resumo(cred, base)

	p.mu.Lock()
	defer p.mu.Unlock()
	if c := p.clientes[conta.ID]; c != nil && c.versao == versao {
		return c, nil
	}
	if cred.ClientID == "" || cred.ClientSecret == "" || len(cred.Certificado) == 0 {
		return nil, fmt.Errorf("efi: a conta %s precisa de client id, client secret e certificado", conta.ID)
	}
	par, err := certificado.TLS(cred.Certificado, cred.SenhaCertificado)
	if err != nil {
		return nil, fmt.Errorf("efi: certificado da conta %s: %w", conta.ID, err)
	}
	mtls := &http.Transport{
		TLSClientConfig:     &tls.Config{Certificates: []tls.Certificate{par}, RootCAs: p.raizes, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	// O token é guardado por versão das credenciais: trocar o secret não reusa o token velho.
	chaveToken := fmt.Sprintf("%s/%x", conta.ID, versao[:8])
	buscar := buscarToken(&http.Client{Transport: mtls, Timeout: p.timeout}, base, cred.ClientID, cred.ClientSecret, p.clock)
	c := &cliente{
		http:   &http.Client{Transport: &oauth.Transport{Base: mtls, Cache: p.tokens, Chave: chaveToken, Buscar: buscar}, Timeout: p.timeout},
		base:   base,
		versao: versao,
	}
	if antigo := p.clientes[conta.ID]; antigo != nil {
		antigo.http.CloseIdleConnections()
	}
	p.clientes[conta.ID] = c
	return c, nil
}

func resumo(cred domain.Credenciais, base string) [32]byte {
	h := sha256.New()
	for _, parte := range [][]byte{[]byte(base), []byte(cred.ClientID), []byte(cred.ClientSecret), cred.Certificado, []byte(cred.SenhaCertificado)} {
		_, _ = fmt.Fprintf(h, "%d:", len(parte))
		_, _ = h.Write(parte)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
