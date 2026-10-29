package app

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Recorrencias cria recorrências de Pix Automático e pede a autorização do
// pagador (RN-40, RN-41). As mudanças de status que chegam do banco ficam para
// o webhook de recorrência.
type Recorrencias struct {
	Store ports.Store
	PSPs  ports.Registry
	Clock ports.Clock
}

// DadosRecorrencia é o payload de recorrencia.criada e recorrencia.atualizada.
type DadosRecorrencia struct {
	IDRec          string `json:"idRec"`
	ContaID        string `json:"contaId"`
	Status         string `json:"status"`
	Contrato       string `json:"contrato"`
	Periodicidade  string `json:"periodicidade"`
	DataInicial    string `json:"dataInicial"`
	MotivoRejeicao string `json:"motivoRejeicao,omitempty"`
}

// DadosSolicitacao é o payload de recorrencia.solicitacao.
type DadosSolicitacao struct {
	IDSolicRec string    `json:"idSolicRec"`
	IDRec      string    `json:"idRec"`
	ContaID    string    `json:"contaId"`
	Status     string    `json:"status"`
	ExpiraEm   time.Time `json:"expiraEm"`
}

func dadosRecorrencia(r domain.Recorrencia) DadosRecorrencia {
	return DadosRecorrencia{
		IDRec: r.ID, ContaID: r.ContaID, Status: string(r.Status), Contrato: r.Contrato,
		Periodicidade: string(r.Periodicidade), DataInicial: r.DataInicial.String(), MotivoRejeicao: r.MotivoRejeicao,
	}
}

// pspRecorrencia encontra o adapter da conta e confere que ele faz Pix
// Automático: declarar CapRec e implementar ports.PSPRecorrencia.
func pspRecorrencia(r ports.Registry, conta domain.Conta) (ports.PSPRecorrencia, error) {
	psp, err := pspDe(r, conta, ports.CapRec)
	if err != nil {
		return nil, err
	}
	rec, ok := psp.(ports.PSPRecorrencia)
	if !ok {
		// Erro de programação do adapter: declara a capacidade sem implementar.
		return nil, fmt.Errorf("%w: o adapter %s declara rec mas não implementa a recorrência", ports.ErrNaoSuportado, conta.PSP)
	}
	return rec, nil
}

// Criar valida a recorrência, envia ao PSP da conta e grava o que o PSP
// devolveu, com o idRec dele, junto com o evento recorrencia.criada.
func (s Recorrencias) Criar(ctx context.Context, contaID string, r domain.Recorrencia) (domain.Recorrencia, error) {
	conta, err := s.Store.Contas().Buscar(ctx, contaID)
	if err != nil {
		return domain.Recorrencia{}, err
	}
	if err := conta.PodeCobrar(); err != nil {
		return domain.Recorrencia{}, err
	}
	r.ContaID, r.Status, r.ID, r.Revisao, r.MotivoRejeicao = conta.ID, domain.RecorrenciaCriada, "", 0, ""
	if err := r.Validar(); err != nil {
		return domain.Recorrencia{}, err
	}
	psp, err := pspRecorrencia(s.PSPs, conta)
	if err != nil {
		return domain.Recorrencia{}, err
	}
	criada, err := psp.CriarRecorrencia(ctx, conta, r)
	if err != nil {
		return domain.Recorrencia{}, erroDoPSP(err)
	}
	if err := domain.ValidarIDRecorrencia(criada.ID); err != nil {
		return domain.Recorrencia{}, fmt.Errorf("%w: o PSP devolveu um idRec inválido: %w", ErrPSP, err)
	}
	criada.ContaID, criada.Revisao = conta.ID, 0
	if criada.Status == "" {
		criada.Status = domain.RecorrenciaCriada
	}
	if criada.CriadaEm.IsZero() {
		criada.CriadaEm = s.Clock.Now()
	}
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Recorrencias().Criar(ctx, criada); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, domain.Evento{
			ID: novoID(), Tipo: domain.EventoRecorrenciaCriada, ContaID: conta.ID, OcorridoEm: s.Clock.Now(), Dados: dadosRecorrencia(criada),
		})
	})
	if err != nil {
		return domain.Recorrencia{}, fmt.Errorf("o PSP criou a recorrência %s, mas ela não foi gravada: %w", criada.ID, err)
	}
	return criada, nil
}

// Consultar devolve a recorrência gravada.
func (s Recorrencias) Consultar(ctx context.Context, idRec string) (domain.Recorrencia, error) {
	return s.Store.Recorrencias().Buscar(ctx, idRec)
}

// idSolicRec segue a regra de formação da spec: SC + ISPB + data + 11 caracteres.
var idSolicRec = regexp.MustCompile(`^SC[0-9A-Z]{8}[0-9]{8}[a-zA-Z0-9]{11}$`)

// SolicitarConfirmacao pede a autorização da recorrência no app do pagador
// (jornada 1). Só recorrência CRIADA pede confirmação.
func (s Recorrencias) SolicitarConfirmacao(ctx context.Context, idRec string, destinatario domain.ContaPagador, expiraEm time.Time) (domain.SolicitacaoRecorrencia, error) {
	rec, err := s.Store.Recorrencias().Buscar(ctx, idRec)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	if rec.Status != domain.RecorrenciaCriada {
		return domain.SolicitacaoRecorrencia{}, fmt.Errorf("%w: a recorrência %s está %s; só CRIADA pede confirmação", domain.ErrTransicaoInvalida, rec.ID, rec.Status)
	}
	conta, err := s.Store.Contas().Buscar(ctx, rec.ContaID)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	sol := domain.SolicitacaoRecorrencia{
		RecorrenciaID: rec.ID, ContaID: conta.ID, Status: domain.SolicitacaoCriada,
		Destinatario: destinatario, ExpiraEm: expiraEm.UTC(), CriadaEm: s.Clock.Now(),
	}
	if err := sol.Validar(s.Clock.Now()); err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	psp, err := pspRecorrencia(s.PSPs, conta)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	criada, err := psp.SolicitarConfirmacao(ctx, conta, sol)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, erroDoPSP(err)
	}
	if !idSolicRec.MatchString(criada.ID) {
		return domain.SolicitacaoRecorrencia{}, fmt.Errorf("%w: o PSP devolveu um idSolicRec inválido: %q", ErrPSP, criada.ID)
	}
	criada.RecorrenciaID, criada.ContaID = rec.ID, conta.ID
	if criada.Status == "" {
		criada.Status = domain.SolicitacaoCriada
	}
	if criada.CriadaEm.IsZero() {
		criada.CriadaEm = s.Clock.Now()
	}
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Solicitacoes().Criar(ctx, criada); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, domain.Evento{
			ID: novoID(), Tipo: domain.EventoSolicitacaoRecorrencia, ContaID: conta.ID, OcorridoEm: s.Clock.Now(),
			Dados: DadosSolicitacao{IDSolicRec: criada.ID, IDRec: rec.ID, ContaID: conta.ID, Status: string(criada.Status), ExpiraEm: criada.ExpiraEm},
		})
	})
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, fmt.Errorf("o PSP criou a solicitação %s, mas ela não foi gravada: %w", criada.ID, err)
	}
	return criada, nil
}
