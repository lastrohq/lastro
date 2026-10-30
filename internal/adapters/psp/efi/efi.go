package efi

import (
	"github.com/lastrohq/lastro/internal/adapters/psp/pixapi"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// URLs da API Pix do Efí por ambiente.
const (
	URLProducao    = "https://pix.api.efipay.com.br"
	URLHomologacao = "https://pix-h.api.efipay.com.br"
)

// PSP é o adapter do Efí: a API Pix da spec (pixapi) com os desvios do Efí.
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
// de cada conta.
func New(credenciais ports.Credenciais, clock ports.Clock, opcoes ...Opcao) *PSP {
	banco := pixapi.Banco{
		Nome:        "efi",
		URLs:        map[domain.Ambiente]string{domain.Producao: URLProducao, domain.Homologacao: URLHomologacao},
		Prefixo:     "/v2",
		BuscarToken: buscarToken,
		Traduzir:    traduzir,
		// O Efí exige mTLS no servidor do webhook; o Lastro não termina TLS e
		// pede o skip-mTLS (veja ConfigurarWebhook em webhook.go).
		HeadersWebhook: map[string]string{"x-skip-mtls-checking": "true"},
	}
	// PUT /v2/cobv/:txid num txid existente sobrescreve a cobv (homologação, 29/10/2026).
	banco.ConsultarAntesDeCriar.Cobv = true
	return &PSP{API: pixapi.New(banco, credenciais, clock, opcoes...)}
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "efi" }

// Capabilities declara o que este adapter já implementa.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapCobv: true, ports.CapDevolucao: true, ports.CapWebhook: true}
}
