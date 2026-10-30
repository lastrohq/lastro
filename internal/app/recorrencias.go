package app

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
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
	if !AcessoDe(ctx).enxerga(contaID) {
		return domain.Recorrencia{}, fmt.Errorf("conta %s: %w", contaID, ports.ErrNaoEncontrado)
	}
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

// Consultar devolve a recorrência gravada. Acesso restrito a outra conta não
// a enxerga: responde ErrNaoEncontrado, sem revelar que ela existe.
func (s Recorrencias) Consultar(ctx context.Context, idRec string) (domain.Recorrencia, error) {
	rec, err := s.Store.Recorrencias().Buscar(ctx, idRec)
	if err != nil {
		return domain.Recorrencia{}, err
	}
	if !AcessoDe(ctx).enxerga(rec.ContaID) {
		return domain.Recorrencia{}, fmt.Errorf("recorrência %s: %w", idRec, ports.ErrNaoEncontrado)
	}
	return rec, nil
}

// idSolicRec segue a regra de formação da spec: SC + ISPB + data + 11 caracteres.
var idSolicRec = regexp.MustCompile(`^SC[0-9A-Z]{8}[0-9]{8}[a-zA-Z0-9]{11}$`)

// SolicitarConfirmacao pede a autorização da recorrência no app do pagador
// (jornada 1). Só recorrência CRIADA pede confirmação.
func (s Recorrencias) SolicitarConfirmacao(ctx context.Context, idRec string, destinatario domain.ContaPagador, expiraEm time.Time) (domain.SolicitacaoRecorrencia, error) {
	rec, err := s.Consultar(ctx, idRec)
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

// ConsultarSolicitacao devolve a solicitação gravada. Acesso restrito a outra
// conta não a enxerga, como na recorrência.
func (s Recorrencias) ConsultarSolicitacao(ctx context.Context, id string) (domain.SolicitacaoRecorrencia, error) {
	sol, err := s.Store.Solicitacoes().Buscar(ctx, id)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	if !AcessoDe(ctx).enxerga(sol.ContaID) {
		return domain.SolicitacaoRecorrencia{}, fmt.Errorf("solicitação %s: %w", id, ports.ErrNaoEncontrado)
	}
	return sol, nil
}

// CancelarSolicitacao desiste do pedido enquanto o pagador não respondeu
// (RN-41). A recorrência continua CRIADA e pode pedir outra confirmação.
func (s Recorrencias) CancelarSolicitacao(ctx context.Context, id string) (domain.SolicitacaoRecorrencia, error) {
	sol, err := s.ConsultarSolicitacao(ctx, id)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	if err := domain.TransicoesSolicitacao.Validar(sol.Status, domain.SolicitacaoCancelada); err != nil {
		return domain.SolicitacaoRecorrencia{}, fmt.Errorf("solicitação %s: %w", id, err)
	}
	conta, err := s.Store.Contas().Buscar(ctx, sol.ContaID)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	psp, err := pspRecorrencia(s.PSPs, conta)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	if _, err := psp.CancelarSolicitacao(ctx, conta, sol); err != nil {
		return domain.SolicitacaoRecorrencia{}, erroDoPSP(err)
	}
	sol.Status = domain.SolicitacaoCancelada
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Solicitacoes().AtualizarStatus(ctx, sol.ID, sol.Status); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, domain.Evento{
			ID: novoID(), Tipo: domain.EventoSolicitacaoRecorrencia, ContaID: sol.ContaID, OcorridoEm: s.Clock.Now(),
			Dados: DadosSolicitacao{IDSolicRec: sol.ID, IDRec: sol.RecorrenciaID, ContaID: sol.ContaID, Status: string(sol.Status), ExpiraEm: sol.ExpiraEm},
		})
	})
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, fmt.Errorf("o PSP cancelou a solicitação %s, mas isso não foi gravado: %w", sol.ID, err)
	}
	return sol, nil
}

// RegistrarAtualizacao aplica a mudança de status que o banco avisou (RN-40):
// grava a recorrência, encerra as solicitações pendentes dela (RN-41) e
// publica recorrencia.atualizada, numa transação só. Devolve se algo mudou.
//
// Aviso repetido (mesmo status) não muda nada. Aviso que a RN-40 não permite,
// como uma volta de APROVADA para CRIADA por aviso fora de ordem, é ignorado
// com registro no log: o banco reenvia o estado atual no próximo aviso.
func (s Recorrencias) RegistrarAtualizacao(ctx context.Context, contaID string, a domain.AtualizacaoRecorrencia) (bool, error) {
	mudou := false
	err := s.Store.WithTx(ctx, func(tx ports.Store) error {
		rec, err := tx.Recorrencias().Buscar(ctx, a.IDRec)
		if err != nil {
			return err
		}
		if rec.ContaID != contaID {
			return fmt.Errorf("recorrência %s de outra conta: %w", a.IDRec, ports.ErrNaoEncontrado)
		}
		if rec.Status == a.Status {
			return nil
		}
		de := rec.Status
		if err := rec.Transitar(a.Status); err != nil {
			slog.WarnContext(ctx, "aviso de recorrência ignorado: transição fora da RN-40", "idRec", rec.ID, "de", de, "para", a.Status)
			return nil
		}
		if a.Status == domain.RecorrenciaRejeitada {
			rec.MotivoRejeicao = a.MotivoRejeicao
		}
		if err := tx.Recorrencias().Atualizar(ctx, &rec); err != nil {
			return err
		}
		if final, ok := domain.SolicitacaoDaRecorrencia(rec.Status); ok {
			sols, err := tx.Solicitacoes().DaRecorrencia(ctx, rec.ID)
			if err != nil {
				return err
			}
			for _, sol := range sols {
				// A recorrência é a fonte da verdade: a solicitação pendente vai
				// direto ao status final, mesmo sem ter passado por RECEBIDA.
				if domain.TransicoesSolicitacao.Terminal(sol.Status) {
					continue
				}
				if err := tx.Solicitacoes().AtualizarStatus(ctx, sol.ID, final); err != nil {
					return err
				}
			}
		}
		mudou = true
		return tx.Outbox().Adicionar(ctx, domain.Evento{
			ID: novoID(), Tipo: domain.EventoRecorrenciaAtualizada, ContaID: rec.ContaID, OcorridoEm: s.Clock.Now(), Dados: dadosRecorrencia(rec),
		})
	})
	return mudou, err
}
