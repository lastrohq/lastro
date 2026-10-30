package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
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

func eventoDevolucao(agora time.Time, tipo domain.TipoEvento, d domain.Devolucao) domain.Evento {
	return domain.Evento{ID: novoID(), Tipo: tipo, ContaID: d.ContaID, OcorridoEm: agora, Dados: DadosDevolucao{
		EndToEndID: d.EndToEndID, ID: d.ID, ContaID: d.ContaID, Valor: d.Valor.String(), Natureza: string(d.Natureza),
		Descricao: d.Descricao, Status: string(d.Status), Motivo: d.Motivo, RtrID: d.RtrID, Revisao: d.Revisao,
	}}
}

// aplicarDevolucao leva para a devolução gravada o que o banco informou sobre
// ela e, se algo mudou, grava com o evento devolucao.atualizada, dentro de tx.
// Devolve se mudou. Devolução que o Vintém não conhece, de outra conta ou com
// status que volta atrás (RN-20) é ignorada com aviso: responder erro faria o
// banco reenviar para sempre. Se o aviso chegou antes de o Solicitar gravar a
// devolução, ela segue EM_PROCESSAMENTO e a conciliação a confere depois.
func aplicarDevolucao(ctx context.Context, tx ports.Store, agora time.Time, doBanco domain.Devolucao) (bool, error) {
	d, err := tx.Devolucoes().Buscar(ctx, doBanco.EndToEndID, doBanco.ID)
	if errors.Is(err, ports.ErrNaoEncontrado) || (err == nil && d.ContaID != doBanco.ContaID) {
		slog.WarnContext(ctx, "aviso de devolução desconhecida ignorado", "conta", doBanco.ContaID, "e2eid", doBanco.EndToEndID, "id", doBanco.ID)
		return false, nil
	}
	if err != nil {
		return false, err
	}
	mudou, _, err := d.Aplicar(doBanco)
	if errors.Is(err, domain.ErrTransicaoInvalida) {
		slog.WarnContext(ctx, "aviso de devolução fora da RN-20 ignorado", "conta", d.ContaID, "e2eid", d.EndToEndID, "id", d.ID, "de", d.Status, "para", doBanco.Status)
		return false, nil
	}
	if err != nil || !mudou {
		return false, err
	}
	if err := tx.Devolucoes().Atualizar(ctx, &d); err != nil {
		return false, err
	}
	return true, tx.Outbox().Adicionar(ctx, eventoDevolucao(agora, domain.EventoDevolucaoAtualizada, d))
}

// travasPix serializa as devoluções de um mesmo Pix nesta instância: duas
// pedidas ao mesmo tempo não podem, juntas, passar do valor recebido. O banco
// confere de novo, mas o Vintém não deve depender disso.
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
		return tx.Outbox().Adicionar(ctx, eventoDevolucao(s.Clock.Now(), domain.EventoDevolucaoSolicitada, d))
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
