package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Devolucoes pede e consulta devoluções de Pix recebidos (RN-20, RN-21). O
// resultado que chega depois do banco fica para o webhook e a conciliação.
type Devolucoes struct {
	Store ports.Store
	PSPs  ports.Registry
	Clock ports.Clock
}

// DadosDevolucao é o payload de devolucao.solicitada e devolucao.atualizada.
type DadosDevolucao struct {
	EndToEndID string `json:"endToEndId"`
	ID         string `json:"id"`
	ContaID    string `json:"contaId"`
	Valor      string `json:"valor"`
	Natureza   string `json:"natureza"`
	Descricao  string `json:"descricao,omitempty"`
	Status     string `json:"status"`
	// Motivo vem quando a devolução não foi realizada.
	Motivo  string `json:"motivo,omitempty"`
	RtrID   string `json:"rtrId,omitempty"`
	Revisao int    `json:"revisao"`
}

func (s Devolucoes) evento(tipo domain.TipoEvento, d domain.Devolucao) domain.Evento {
	return domain.Evento{ID: novoID(), Tipo: tipo, ContaID: d.ContaID, OcorridoEm: s.Clock.Now(), Dados: DadosDevolucao{
		EndToEndID: d.EndToEndID, ID: d.ID, ContaID: d.ContaID, Valor: d.Valor.String(), Natureza: string(d.Natureza),
		Descricao: d.Descricao, Status: string(d.Status), Motivo: d.Motivo, RtrID: d.RtrID, Revisao: d.Revisao,
	}}
}

// travasPix serializa as devoluções de um mesmo Pix nesta instância: duas
// pedidas ao mesmo tempo não podem, juntas, passar do valor recebido. O banco
// confere de novo, mas o Lastro não deve depender disso.
var travasPix sync.Map // e2e → *sync.Mutex

func travarPix(e2e string) func() {
	m, _ := travasPix.LoadOrStore(e2e, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// Solicitar confere a devolução contra o Pix (RN-21), pede ao PSP da conta e
// grava o que ele devolveu, com o evento devolucao.solicitada. O par e2e + id é
// idempotente: repetir com o mesmo valor devolve a devolução existente sem
// chamar o banco; com outro valor, ErrJaExiste. Natureza vazia é ORIGINAL.
//
// Conta desativada continua devolvendo: a devolução é de um Pix que já entrou.
func (s Devolucoes) Solicitar(ctx context.Context, d domain.Devolucao) (domain.Devolucao, error) {
	if err := domain.ValidarEndToEndID(d.EndToEndID); err != nil {
		return domain.Devolucao{}, &ErroValidacao{Violacoes: []Violacao{{Propriedade: "e2eid", Razao: err.Error(), Valor: d.EndToEndID}}}
	}
	if err := domain.ValidarIDDevolucao(d.ID); err != nil {
		return domain.Devolucao{}, err
	}
	pix, err := s.Store.Pix().BuscarPorE2E(ctx, d.EndToEndID)
	if err != nil {
		return domain.Devolucao{}, err
	}
	if !AcessoDe(ctx).enxerga(pix.ContaID) {
		return domain.Devolucao{}, fmt.Errorf("pix %s: %w", d.EndToEndID, ports.ErrNaoEncontrado)
	}
	destravar := travarPix(d.EndToEndID)
	defer destravar()

	if d.Natureza == "" {
		d.Natureza = domain.DevolucaoOriginal
	}
	existente, err := s.Store.Devolucoes().Buscar(ctx, d.EndToEndID, d.ID)
	switch {
	case err == nil:
		if existente.Valor != d.Valor || existente.Natureza != d.Natureza {
			return domain.Devolucao{}, fmt.Errorf("devolução %s do Pix %s já existe com valor %s: %w", d.ID, d.EndToEndID, existente.Valor, ports.ErrJaExiste)
		}
		return existente, nil
	case !errors.Is(err, ports.ErrNaoEncontrado):
		return domain.Devolucao{}, err
	}
	anteriores, err := s.Store.Devolucoes().DoPix(ctx, d.EndToEndID)
	if err != nil {
		return domain.Devolucao{}, err
	}
	agora := s.Clock.Now()
	d.ContaID, d.Status, d.Motivo, d.RtrID, d.LiquidadaEm, d.Revisao = pix.ContaID, domain.DevolucaoEmProcessamento, "", "", time.Time{}, 0
	d.CriadaEm = agora
	if err := d.ValidarSolicitacao(pix, anteriores, agora); err != nil {
		return domain.Devolucao{}, err
	}
	conta, err := s.Store.Contas().Buscar(ctx, pix.ContaID)
	if err != nil {
		return domain.Devolucao{}, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapDevolucao)
	if err != nil {
		return domain.Devolucao{}, err
	}
	feita, err := psp.Devolver(ctx, conta, d)
	if err != nil {
		return domain.Devolucao{}, erroDoPSP(err)
	}
	if feita.ID != d.ID || feita.Valor != d.Valor {
		return domain.Devolucao{}, fmt.Errorf("%w: o PSP devolveu a devolução %q de %s para o pedido %s de %s", ErrPSP, feita.ID, feita.Valor, d.ID, d.Valor)
	}
	switch feita.Status {
	case "", domain.DevolucaoEmProcessamento:
	case domain.DevolucaoDevolvido, domain.DevolucaoNaoRealizado:
		d.Status = feita.Status
	default:
		return domain.Devolucao{}, fmt.Errorf("%w: status de devolução desconhecido %q", ErrPSP, feita.Status)
	}
	d.Motivo, d.RtrID, d.LiquidadaEm = feita.Motivo, feita.RtrID, feita.LiquidadaEm
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Devolucoes().Criar(ctx, d); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(domain.EventoDevolucaoSolicitada, d))
	})
	if err != nil {
		return domain.Devolucao{}, fmt.Errorf("o PSP aceitou a devolução %s do Pix %s, mas ela não foi gravada: %w", d.ID, d.EndToEndID, err)
	}
	return d, nil
}

// Consultar devolve a devolução gravada. Acesso restrito a outra conta não a
// enxerga: responde ErrNaoEncontrado, sem revelar que ela existe.
func (s Devolucoes) Consultar(ctx context.Context, e2e, id string) (domain.Devolucao, error) {
	d, err := s.Store.Devolucoes().Buscar(ctx, e2e, id)
	if err != nil {
		return domain.Devolucao{}, err
	}
	if !AcessoDe(ctx).enxerga(d.ContaID) {
		return domain.Devolucao{}, fmt.Errorf("devolução %s do Pix %s: %w", id, e2e, ports.ErrNaoEncontrado)
	}
	return d, nil
}
