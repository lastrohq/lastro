package sandbox

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

var _ ports.PSPRecorrencia = (*PSP)(nil)

// ispbSandbox é o ISPB fictício que o sandbox usa nos ids de Pix Automático.
const ispbSandbox = "00000000"

const alfanumericos = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func sufixo11() string {
	b := make([]byte, 11)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alfanumericos))))
		b[i] = alfanumericos[n.Int64()]
	}
	return string(b)
}

// CriarRecorrencia registra a recorrência com um idRec no formato da spec:
// R + R/N (retentativa) + ISPB + data + 11 caracteres.
func (p *PSP) CriarRecorrencia(_ context.Context, conta domain.Conta, r domain.Recorrencia) (domain.Recorrencia, error) {
	tentativa := "N"
	if r.PermiteRetentativa() {
		tentativa = "R"
	}
	if r.CriadaEm.IsZero() {
		r.CriadaEm = p.clock.Now()
	}
	r.ID = "R" + tentativa + ispbSandbox + r.CriadaEm.UTC().Format("20060102") + sufixo11()
	r.ContaID, r.Status = conta.ID, domain.RecorrenciaCriada
	p.mu.Lock()
	defer p.mu.Unlock()
	p.recs[r.ID] = r
	return r, nil
}

// ConsultarRecorrencia devolve a recorrência da memória ou, depois de um
// restart, a que o Lastro gravou.
func (p *PSP) ConsultarRecorrencia(ctx context.Context, conta domain.Conta, id string) (domain.Recorrencia, error) {
	p.mu.Lock()
	r, ok := p.recs[id]
	p.mu.Unlock()
	if !ok && p.store != nil {
		var err error
		if r, err = p.store.Recorrencias().Buscar(ctx, id); err == nil {
			ok = true
		} else if !errors.Is(err, ports.ErrNaoEncontrado) {
			return domain.Recorrencia{}, fmt.Errorf("sandbox: lembrando a recorrência %s: %w", id, err)
		}
	}
	if !ok || r.ContaID != conta.ID {
		return domain.Recorrencia{}, fmt.Errorf("sandbox: recorrência %s: %w", id, ports.ErrNaoEncontrado)
	}
	return r, nil
}

// SolicitarConfirmacao registra a solicitação como ENVIADA ao app do pagador.
func (p *PSP) SolicitarConfirmacao(ctx context.Context, conta domain.Conta, s domain.SolicitacaoRecorrencia) (domain.SolicitacaoRecorrencia, error) {
	if _, err := p.ConsultarRecorrencia(ctx, conta, s.RecorrenciaID); err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	if s.CriadaEm.IsZero() {
		s.CriadaEm = p.clock.Now()
	}
	s.ID = "SC" + ispbSandbox + s.CriadaEm.UTC().Format("20060102") + sufixo11()
	s.ContaID, s.Status = conta.ID, domain.SolicitacaoEnviada
	return s, nil
}
