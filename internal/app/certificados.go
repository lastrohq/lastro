package app

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Certificados avisa a aplicação do cliente quando o certificado do mTLS de uma
// conta está para vencer (RN-62).
type Certificados struct {
	Store ports.Store
	Clock ports.Clock
}

// DadosCertificado é o payload de certificado.expirando. Nunca leva o
// certificado, só os dados públicos dele.
type DadosCertificado struct {
	ContaID          string    `json:"contaId"`
	Sujeito          string    `json:"sujeito"`
	ImpressaoDigital string    `json:"impressaoDigital"`
	Validade         time.Time `json:"validade"`
	// DiasRestantes arredonda para cima; zero ou negativo quando já venceu.
	DiasRestantes int  `json:"diasRestantes"`
	Vencido       bool `json:"vencido"`
}

// AvisarVencimentos publica certificado.expirando para cada conta ativa cujo
// certificado vence em até 30 dias (ou já venceu) e ainda não foi avisado. O
// evento e a marca de avisado entram na mesma transação: rodar de novo não
// repete o aviso. Devolve quantos avisos saíram.
func (s Certificados) AvisarVencimentos(ctx context.Context) (int, error) {
	contas, err := s.Store.Contas().Listar(ctx)
	if err != nil {
		return 0, err
	}
	agora := s.Clock.Now()
	avisos := 0
	var erros []error
	for _, c := range contas {
		if !c.Ativa || c.Certificado == nil || !c.Certificado.PrecisaAvisarEm(agora) {
			continue
		}
		avisou, err := s.avisar(ctx, c, agora)
		if err != nil {
			// Uma conta que falha não impede as outras.
			erros = append(erros, fmt.Errorf("conta %s: %w", c.ID, err))
			continue
		}
		if avisou {
			avisos++
		}
	}
	return avisos, errors.Join(erros...)
}

// avisar publica o evento e marca o certificado como avisado. Devolve false se,
// dentro da transação, o certificado já estava avisado ou tinha sido trocado.
func (s Certificados) avisar(ctx context.Context, c domain.Conta, agora time.Time) (bool, error) {
	cert := *c.Certificado
	restante := cert.Validade.Sub(agora)
	dados := DadosCertificado{
		ContaID:          c.ID,
		Sujeito:          cert.Sujeito,
		ImpressaoDigital: cert.ImpressaoDigital,
		Validade:         cert.Validade,
		DiasRestantes:    int(math.Ceil(restante.Hours() / 24)),
		Vencido:          cert.VencidoEm(agora),
	}
	avisou := false
	err := s.Store.WithTx(ctx, func(tx ports.Store) error {
		atual, err := tx.Contas().Buscar(ctx, c.ID)
		if err != nil {
			return err
		}
		if atual.Certificado == nil || atual.Certificado.ImpressaoDigital != cert.ImpressaoDigital || atual.Certificado.VencimentoAvisado {
			return nil
		}
		if err := tx.Contas().MarcarVencimentoAvisado(ctx, c.ID, cert.ImpressaoDigital); err != nil {
			return err
		}
		avisou = true
		return tx.Outbox().Adicionar(ctx, domain.Evento{
			ID: novoID(), Tipo: domain.EventoCertificadoVencendo, ContaID: c.ID, OcorridoEm: agora, Dados: dados,
		})
	})
	return avisou && err == nil, err
}
