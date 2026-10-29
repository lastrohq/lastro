package sandbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

var _ ports.PSPCobrancaRecorrente = (*PSP)(nil)

// Valores mágicos da cobrança recorrente, em centavos. Como a cobrança tem o
// valor fixo da recorrência, é a recorrência que escolhe o comportamento.
const (
	// ValorCobrRejeitada faz o PSP do pagador rejeitar a instrução de pagamento.
	ValorCobrRejeitada domain.Money = 44
	// ValorCobrSemSaldo faz a tentativa do vencimento falhar por falta de saldo;
	// a retentativa é paga.
	ValorCobrSemSaldo domain.Money = 55
)

// O sandbox comprime o calendário: em vez de esperar o dia da liquidação, a
// cobrança anda um passo a cada atraso. Depois de um atraso o PSP do pagador
// aceita a instrução (ATIVA); depois de dois, a tentativa do vencimento é
// liquidada. Cada retentativa é liquidada um atraso depois de pedida.

// cobrSandbox é o que o sandbox lembra de uma cobrança recorrente: como ela
// foi criada e o que o recebedor pediu depois. O status é calculado na hora.
type cobrSandbox struct {
	c            domain.CobrancaRecorrente
	retentativas []retentativa
	// canceladaEm congela a cobrança: dali em diante nada é liquidado.
	canceladaEm time.Time
}

type retentativa struct {
	data   domain.Data
	pedida time.Time
}

// e2eSandbox monta um endToEndId no formato da spec: E + ISPB + AAAAMMDDHHmm + 11.
func e2eSandbox(d domain.Data, txid domain.Txid) string {
	return "E" + ispbSandbox + d.Tempo().Format("20060102") + "1200" + string(txid)[:11]
}

// estado calcula a cobrança no instante agora.
func (p *PSP) estado(s cobrSandbox, agora time.Time) domain.CobrancaRecorrente {
	c := s.c
	c.Tentativas, c.MotivoEncerramento = nil, ""
	cancelada := !s.canceladaEm.IsZero()
	if cancelada && agora.After(s.canceladaEm) {
		agora = s.canceladaEm
	}
	passou := func(desde time.Time, n int) bool { return !agora.Before(desde.Add(time.Duration(n) * p.atraso)) }
	switch {
	case !passou(c.CriadaEm, 1):
		c.Status = domain.CobrCriada
	case c.Valor == ValorCobrRejeitada:
		c.Status, c.MotivoEncerramento = domain.CobrRejeitada, "AC06 instrução rejeitada pelo PSP do pagador (sandbox)"
	default:
		c.Status = domain.CobrAtiva
		agnd := domain.Tentativa{DataLiquidacao: c.Vencimento, Tipo: domain.TipoAgendamento, Status: domain.TentativaAgendada}
		if passou(c.CriadaEm, 2) {
			if c.Valor == ValorCobrSemSaldo {
				agnd.Status, agnd.MotivoRejeicao = domain.TentativaRejeitada, "AM04 saldo insuficiente (sandbox)"
			} else {
				agnd.Status, agnd.EndToEndID, c.Status = domain.TentativaPaga, e2eSandbox(agnd.DataLiquidacao, c.Txid), domain.CobrConcluida
			}
		}
		c.Tentativas = append(c.Tentativas, agnd)
		for _, r := range s.retentativas {
			nova := domain.Tentativa{DataLiquidacao: r.data, Tipo: domain.TipoNovaTentativa, Status: domain.TentativaAgendada}
			if c.Status == domain.CobrAtiva && passou(r.pedida, 1) {
				nova.Status, nova.EndToEndID, c.Status = domain.TentativaPaga, e2eSandbox(r.data, c.Txid), domain.CobrConcluida
			}
			c.Tentativas = append(c.Tentativas, nova)
		}
	}
	if cancelada && !domain.TransicoesCobrancaRecorrente.Terminal(c.Status) {
		c.Status, c.MotivoEncerramento = domain.CobrCancelada, "SLCR cancelada pelo recebedor (sandbox)"
		for i, t := range c.Tentativas {
			if t.Status == domain.TentativaAgendada || t.Status == domain.TentativaSolicitada {
				c.Tentativas[i].Status = domain.TentativaCancelada
			}
		}
	}
	return c
}

// CriarCobr registra a cobrança da recorrência da conta, com status CRIADA.
func (p *PSP) CriarCobr(ctx context.Context, conta domain.Conta, c domain.CobrancaRecorrente) (domain.CobrancaRecorrente, error) {
	if c.Valor == ValorRecusado {
		return domain.CobrancaRecorrente{}, ErrRecusado
	}
	if _, err := p.ConsultarRecorrencia(ctx, conta, c.RecorrenciaID); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if c.CriadaEm.IsZero() {
		c.CriadaEm = p.clock.Now()
	}
	c.ContaID, c.Status, c.Tentativas = conta.ID, domain.CobrCriada, nil
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ja := p.cobrs[chave(conta.ID, c.Txid)]; ja {
		return domain.CobrancaRecorrente{}, fmt.Errorf("sandbox: cobrança %s: %w", c.Txid, ports.ErrJaExiste)
	}
	p.cobrs[chave(conta.ID, c.Txid)] = cobrSandbox{c: c}
	return c, nil
}

// lembrarCobr devolve a cobrança da memória ou, depois de um restart, a
// reconstrói da que o Lastro gravou: as retentativas gravadas contam como
// pedidas na criação, e a cobrança cancelada continua cancelada.
func (p *PSP) lembrarCobr(ctx context.Context, conta domain.Conta, txid domain.Txid) (cobrSandbox, error) {
	p.mu.Lock()
	s, ok := p.cobrs[chave(conta.ID, txid)]
	p.mu.Unlock()
	if ok {
		return s, nil
	}
	if p.store != nil {
		c, err := p.store.CobrancasRecorrentes().BuscarPorTxid(ctx, txid)
		if err != nil && !errors.Is(err, ports.ErrNaoEncontrado) {
			return cobrSandbox{}, fmt.Errorf("sandbox: lembrando a cobrança %s: %w", txid, err)
		}
		if err == nil && c.ContaID == conta.ID {
			s = cobrSandbox{c: c}
			if c.Status == domain.CobrCancelada {
				s.canceladaEm = c.CriadaEm
			}
			for _, t := range c.Retentativas() {
				s.retentativas = append(s.retentativas, retentativa{data: t, pedida: c.CriadaEm})
			}
			s.c.Status, s.c.Tentativas, s.c.MotivoEncerramento = domain.CobrCriada, nil, ""
			return s, nil
		}
	}
	return cobrSandbox{}, fmt.Errorf("sandbox: cobrança recorrente %s: %w", txid, ports.ErrNaoEncontrado)
}

// ConsultarCobr devolve a cobrança como ela está agora.
func (p *PSP) ConsultarCobr(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	s, err := p.lembrarCobr(ctx, conta, txid)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	return p.estado(s, p.clock.Now()), nil
}

// CancelarCobr cancela a cobrança que ainda não terminou.
func (p *PSP) CancelarCobr(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	return p.mudarCobr(ctx, conta, txid, func(s *cobrSandbox, atual domain.CobrancaRecorrente) error {
		if err := domain.TransicoesCobrancaRecorrente.Validar(atual.Status, domain.CobrCancelada); err != nil {
			return fmt.Errorf("sandbox: %w", err)
		}
		s.canceladaEm = p.clock.Now()
		return nil
	})
}

// SolicitarRetentativa agenda uma nova data para a cobrança ATIVA.
func (p *PSP) SolicitarRetentativa(ctx context.Context, conta domain.Conta, txid domain.Txid, data domain.Data) (domain.CobrancaRecorrente, error) {
	return p.mudarCobr(ctx, conta, txid, func(s *cobrSandbox, atual domain.CobrancaRecorrente) error {
		if atual.Status != domain.CobrAtiva {
			return fmt.Errorf("sandbox: %w: a cobrança está %s; só ATIVA ganha nova data", domain.ErrTransicaoInvalida, atual.Status)
		}
		s.retentativas = append(s.retentativas, retentativa{data: data, pedida: p.clock.Now()})
		return nil
	})
}

func (p *PSP) mudarCobr(ctx context.Context, conta domain.Conta, txid domain.Txid, mudar func(*cobrSandbox, domain.CobrancaRecorrente) error) (domain.CobrancaRecorrente, error) {
	s, err := p.lembrarCobr(ctx, conta, txid)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if err := mudar(&s, p.estado(s, p.clock.Now())); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	p.mu.Lock()
	p.cobrs[chave(conta.ID, txid)] = s
	p.mu.Unlock()
	return p.estado(s, p.clock.Now()), nil
}
