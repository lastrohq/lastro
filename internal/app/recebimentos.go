package app

import (
	"context"
	"errors"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Recebimentos registra os Pix que entraram na conta, venham do webhook do
// banco ou da conciliação (RN-30, RN-31).
type Recebimentos struct {
	Store ports.Store
	Clock ports.Clock
}

// DadosPix é o payload normalizado do evento pix.recebido (RN-32).
type DadosPix struct {
	EndToEndID string    `json:"endToEndId"`
	ContaID    string    `json:"contaId"`
	Txid       string    `json:"txid,omitempty"`
	Valor      string    `json:"valor"`
	Horario    time.Time `json:"horario"`
	// Divergente avisa que o valor pago difere do valor da cobrança.
	Divergente bool `json:"divergente,omitempty"`
}

// Registrar grava cada Pix uma vez só (pelo endToEndId) e, se ele pertence a
// uma cobrança ativa da mesma conta, conclui a cobrança. As devoluções que o
// banco informa junto com o Pix, novo ou não, atualizam as gravadas (RN-20).
// Tudo numa transação:
// ou o lote inteiro entra com seus eventos, ou nada entra e o banco reenvia.
// Devolve quantos Pix eram novos.
func (s Recebimentos) Registrar(ctx context.Context, pixes []domain.PixRecebido) (int, error) {
	novos := 0
	err := s.Store.WithTx(ctx, func(tx ports.Store) error {
		novos = 0
		for _, p := range pixes {
			novo, err := tx.Pix().Registrar(ctx, p)
			if err != nil {
				return err
			}
			for _, d := range p.Devolucoes {
				d.EndToEndID, d.ContaID = p.EndToEndID, p.ContaID
				if _, err := aplicarDevolucao(ctx, tx, s.Clock.Now(), d); err != nil {
					return err
				}
			}
			if !novo {
				continue
			}
			novos++
			dados := DadosPix{EndToEndID: p.EndToEndID, ContaID: p.ContaID, Txid: string(p.Txid), Valor: p.Valor.String(), Horario: p.Horario}
			if p.Txid != "" {
				divergente, err := s.concluir(ctx, tx, p)
				if err != nil {
					return err
				}
				dados.Divergente = divergente
			}
			if err := tx.Outbox().Adicionar(ctx, domain.Evento{ID: novoID(), Tipo: domain.EventoPixRecebido, ContaID: p.ContaID, OcorridoEm: s.Clock.Now(), Dados: dados}); err != nil {
				return err
			}
		}
		return nil
	})
	return novos, err
}

// concluir leva a cob ou cobv do txid a CONCLUIDA. O banco é a fonte da verdade:
// valor diferente do cobrado ainda conclui, mas o evento sai com divergente=true.
func (s Recebimentos) concluir(ctx context.Context, tx ports.Store, p domain.PixRecebido) (bool, error) {
	cob, err := tx.Cobrancas().BuscarPorTxid(ctx, p.Txid)
	switch {
	case err == nil:
		if cob.ContaID != p.ContaID || cob.Status != domain.CobrancaAtiva {
			return false, nil
		}
		divergente := cob.Valor != p.Valor && !cob.PermiteAlteracao
		_ = cob.Transitar(domain.CobrancaConcluida)
		if err := tx.Cobrancas().Atualizar(ctx, &cob); err != nil {
			return false, err
		}
		return divergente, tx.Outbox().Adicionar(ctx, domain.Evento{ID: novoID(), Tipo: domain.EventoCobrancaConcluida, ContaID: cob.ContaID, OcorridoEm: s.Clock.Now(),
			Dados: DadosCobranca{Tipo: "cob", Txid: string(cob.Txid), ContaID: cob.ContaID, Status: string(cob.Status), Valor: cob.Valor.String(), Revisao: cob.Revisao}})
	case !errors.Is(err, ports.ErrNaoEncontrado):
		return false, err
	}
	v, err := tx.CobrancasV().BuscarPorTxid(ctx, p.Txid)
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return false, nil // Pix com txid que não é nosso: registrado, sem cobrança para concluir
	}
	if err != nil || v.ContaID != p.ContaID || v.Status != domain.CobrancaAtiva {
		return false, err
	}
	// Na cobv o valor final depende de multa, juros e desconto, calculados pelo banco:
	// não há como dizer aqui se é divergente.
	_ = v.Transitar(domain.CobrancaConcluida)
	if err := tx.CobrancasV().Atualizar(ctx, &v); err != nil {
		return false, err
	}
	return false, tx.Outbox().Adicionar(ctx, domain.Evento{ID: novoID(), Tipo: domain.EventoCobrancaConcluida, ContaID: v.ContaID, OcorridoEm: s.Clock.Now(),
		Dados: DadosCobranca{Tipo: "cobv", Txid: string(v.Txid), ContaID: v.ContaID, Status: string(v.Status), Valor: v.Valor.String(), Revisao: v.Revisao}})
}

// DaCobranca devolve os Pix recebidos para o txid, respeitando o acesso por conta.
func (s Recebimentos) DaCobranca(ctx context.Context, contaID string, txid domain.Txid) ([]domain.PixRecebido, error) {
	pixes, err := s.Store.Pix().ListarPorTxid(ctx, txid)
	if err != nil {
		return nil, err
	}
	out := pixes[:0]
	for _, p := range pixes {
		if p.ContaID == contaID {
			out = append(out, p)
		}
	}
	return out, nil
}
