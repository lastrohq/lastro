// Package sandbox é o PSP de mentira do Lastro: implementa todas as
// capacidades sem banco nenhum, para testar o fluxo inteiro sem conta PJ (RN-52).
// É também o adapter de referência da suíte de conformidade.
//
// O comportamento é determinístico e controlado pelo valor da cobrança
// (os "valores mágicos", como os cartões de teste dos gateways):
//
//	R$ 0,66  o PSP recusa a criação (erro do banco)
//	R$ 0,99  nunca é paga: a cobrança expira
//	R$ 1,01  é paga com valor diferente (R$ 1,00), para testar divergência
//	R$ 2,02  é paga duas vezes (webhook duplicado)
//	outros   é paga integralmente depois do atraso configurado
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/pkg/brcode"
)

// Valores mágicos, em centavos.
const (
	ValorRecusado   domain.Money = 66
	ValorNuncaPaga  domain.Money = 99
	ValorDivergente domain.Money = 101
	ValorDuplicado  domain.Money = 202
)

// ErrRecusado é o erro que o sandbox devolve para ValorRecusado.
var ErrRecusado = errors.New("sandbox: o PSP recusou a cobrança (valor mágico 0.66)")

// AtrasoPadrao é quanto tempo a cobrança leva para ser paga.
const AtrasoPadrao = 5 * time.Second

// PSP é o adapter sandbox. O estado vive em memória.
type PSP struct {
	clock  ports.Clock
	atraso time.Duration

	mu         sync.Mutex
	cobs       map[string]domain.Cobranca // chave: contaID/txid
	devolucoes map[string][]domain.Devolucao
	webhooks   map[string]string
	locSeq     int
}

var _ ports.PSP = (*PSP)(nil)

// Opcao configura o sandbox.
type Opcao func(*PSP)

// ComAtraso define em quanto tempo a cobrança é paga.
func ComAtraso(d time.Duration) Opcao { return func(p *PSP) { p.atraso = d } }

// New cria o sandbox com o relógio dado.
func New(clock ports.Clock, opcoes ...Opcao) *PSP {
	p := &PSP{clock: clock, atraso: AtrasoPadrao, cobs: map[string]domain.Cobranca{}, devolucoes: map[string][]domain.Devolucao{}, webhooks: map[string]string{}}
	for _, o := range opcoes {
		o(p)
	}
	return p
}

// Nome identifica o adapter no registry.
func (p *PSP) Nome() string { return "sandbox" }

// Capabilities declara tudo o que o sandbox simula hoje.
func (p *PSP) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapDevolucao: true, ports.CapWebhook: true}
}

func chave(contaID string, txid domain.Txid) string { return contaID + "/" + string(txid) }

// CriarCob registra a cobrança e devolve location e BR Code dinâmico.
func (p *PSP) CriarCob(_ context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	if c.Valor == ValorRecusado {
		return domain.Cobranca{}, ErrRecusado
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	k := chave(conta.ID, c.Txid)
	if _, ok := p.cobs[k]; ok {
		return domain.Cobranca{}, fmt.Errorf("sandbox: txid %s: %w", c.Txid, ports.ErrJaExiste)
	}
	p.locSeq++
	c.ContaID = conta.ID
	c.Status = domain.CobrancaAtiva
	c.Location = fmt.Sprintf("sandbox.lastro.dev/qr/v2/%s/%d", conta.ID, p.locSeq)
	if c.CriadaEm.IsZero() {
		c.CriadaEm = p.clock.Now()
	}
	code, err := brcode.Encode(brcode.Pix{URL: c.Location, Nome: nomeBRCode(conta.Nome), Cidade: "SANDBOX", Valor: int64(c.Valor)})
	if err != nil {
		return domain.Cobranca{}, fmt.Errorf("sandbox: gerando BR Code: %w", err)
	}
	c.BRCode = code
	p.cobs[k] = c
	return c, nil
}

// ConsultarCob devolve a cobrança, já CONCLUIDA se o atraso passou.
func (p *PSP) ConsultarCob(_ context.Context, conta domain.Conta, txid domain.Txid) (domain.Cobranca, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.cobs[chave(conta.ID, txid)]
	if !ok {
		return domain.Cobranca{}, fmt.Errorf("sandbox: txid %s: %w", txid, ports.ErrNaoEncontrado)
	}
	if p.paga(c) {
		c.Status = domain.CobrancaConcluida
	}
	return c, nil
}

// paga diz se a cobrança já foi "paga" no instante atual do relógio.
func (p *PSP) paga(c domain.Cobranca) bool {
	if c.Valor == ValorNuncaPaga || c.Status != domain.CobrancaAtiva {
		return false
	}
	pagoEm := c.CriadaEm.Add(p.atraso)
	return !p.clock.Now().Before(pagoEm) && pagoEm.Before(c.CriadaEm.Add(c.Expiracao))
}

// ListarPix devolve os pagamentos simulados cujo horário cai na janela.
func (p *PSP) ListarPix(_ context.Context, conta domain.Conta, j ports.Janela) ([]domain.PixRecebido, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []domain.PixRecebido
	for _, c := range p.cobs {
		if c.ContaID != conta.ID || !p.paga(c) {
			continue
		}
		horario := c.CriadaEm.Add(p.atraso)
		if horario.Before(j.Inicio) || !horario.Before(j.Fim) {
			continue
		}
		valor := c.Valor
		if valor == ValorDivergente {
			valor = 100
		}
		vezes := 1
		if c.Valor == ValorDuplicado {
			vezes = 2
		}
		pix := domain.PixRecebido{EndToEndID: endToEndID(horario, c.Txid), ContaID: conta.ID, Txid: c.Txid, Valor: valor, Horario: horario, Pagador: &domain.Devedor{Documento: "12345678909", Nome: "Pagador Sandbox"}}
		for range vezes {
			out = append(out, pix)
		}
	}
	return out, nil
}

// Devolver aceita devoluções até o valor do Pix e responde DEVOLVIDO na hora.
func (p *PSP) Devolver(ctx context.Context, conta domain.Conta, d domain.Devolucao) (domain.Devolucao, error) {
	pixes, err := p.ListarPix(ctx, conta, ports.Janela{Fim: p.clock.Now().Add(time.Nanosecond)})
	if err != nil {
		return domain.Devolucao{}, err
	}
	var recebido domain.Money
	for _, px := range pixes {
		if px.EndToEndID == d.EndToEndID {
			recebido = px.Valor
			break
		}
	}
	if recebido == 0 {
		return domain.Devolucao{}, fmt.Errorf("sandbox: pix %s: %w", d.EndToEndID, ports.ErrNaoEncontrado)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var devolvido domain.Money
	for _, ant := range p.devolucoes[d.EndToEndID] {
		if ant.ID == d.ID {
			return ant, nil // idempotente pelo id
		}
		devolvido += ant.Valor
	}
	if devolvido+d.Valor > recebido {
		return domain.Devolucao{}, fmt.Errorf("sandbox: devolução de %s excede o saldo de %s", d.Valor, recebido-devolvido)
	}
	d.ContaID, d.Status, d.CriadaEm = conta.ID, domain.DevolucaoDevolvido, p.clock.Now()
	p.devolucoes[d.EndToEndID] = append(p.devolucoes[d.EndToEndID], d)
	return d, nil
}

// ConfigurarWebhook guarda a URL. A entrega dos avisos chega com a F4.
func (p *PSP) ConfigurarWebhook(_ context.Context, conta domain.Conta, url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.webhooks[conta.ID] = url
	return nil
}

// endToEndID segue o formato do SPI: E + ISPB(8) + AAAAMMDDHHmm + 11 caracteres.
// É derivado do txid para que consultas repetidas devolvam o mesmo id.
func endToEndID(t time.Time, txid domain.Txid) string {
	sufixo := strings.ToUpper(string(txid))
	if len(sufixo) > 11 {
		sufixo = sufixo[len(sufixo)-11:]
	}
	for len(sufixo) < 11 {
		sufixo = "0" + sufixo
	}
	return "E00000000" + t.UTC().Format("200601021504") + sufixo
}

func nomeBRCode(nome string) string {
	if len(nome) > 25 {
		return nome[:25]
	}
	if nome == "" {
		return "LASTRO SANDBOX"
	}
	return nome
}
