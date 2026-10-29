package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// JanelaConciliacaoPadrao é quanto tempo para trás a conciliação olha (RN-31).
const JanelaConciliacaoPadrao = 2 * time.Hour

// Conciliacao recupera os Pix que o banco recebeu mas não avisou (RN-31) e,
// nas contas com Pix Automático, as mudanças de recorrência sem aviso (RN-40).
type Conciliacao struct {
	Store        ports.Store
	PSPs         ports.Registry
	Recebimentos Recebimentos
	// Recorrencias vazia (Store nil) pula a conciliação de recorrências.
	Recorrencias Recorrencias
	// CobrancasRecorrentes vazia (Store nil) pula a das cobranças recorrentes.
	CobrancasRecorrentes CobrancasRecorrentes
	Clock                ports.Clock
	// Janela é quanto tempo para trás consultar; zero usa JanelaConciliacaoPadrao.
	Janela time.Duration
}

// ResultadoConciliacao resume uma rodada.
type ResultadoConciliacao struct {
	Contas int
	// Novos são os Pix que não tinham chegado por webhook.
	Novos int
	// Recorrencias são as recorrências que mudaram sem aviso.
	Recorrencias int
	// CobrancasRecorrentes são as cobranças recorrentes que mudaram sem aviso.
	CobrancasRecorrentes int
	Falhas               int
}

// Rodar consulta GET /pix de cada conta na janela [agora-Janela, agora) e
// registra o que faltar. Um Pix no instante exato de agora fica para a próxima
// rodada, que o cobre porque as janelas se sobrepõem.
// Conta desativada também entra: as cobranças antigas dela ainda podem ser pagas.
// Uma conta que falha não impede as outras.
func (s Conciliacao) Rodar(ctx context.Context) (ResultadoConciliacao, error) {
	contas, err := s.Store.Contas().Listar(ctx)
	if err != nil {
		return ResultadoConciliacao{}, err
	}
	janela := s.Janela
	if janela <= 0 {
		janela = JanelaConciliacaoPadrao
	}
	agora := s.Clock.Now()
	var r ResultadoConciliacao
	for _, conta := range contas {
		r.Contas++
		psp, err := s.PSPs.PSP(conta.PSP)
		if err != nil {
			r.Falhas++
			slog.ErrorContext(ctx, "conciliação: conta com PSP desconhecido", "conta", conta.ID, "psp", conta.PSP, "erro", err)
			continue
		}
		// As recorrências não dependem de a consulta de Pix dar certo.
		mudaram, falhas := s.conciliarRecorrencias(ctx, conta)
		r.Recorrencias += mudaram
		r.Falhas += falhas
		mudaram, falhas = s.conciliarCobrs(ctx, conta)
		r.CobrancasRecorrentes += mudaram
		r.Falhas += falhas
		pixes, err := psp.ListarPix(ctx, conta, ports.Janela{Inicio: agora.Add(-janela), Fim: agora})
		if err != nil {
			r.Falhas++
			slog.WarnContext(ctx, "conciliação: consulta ao PSP falhou", "conta", conta.ID, "psp", conta.PSP, "erro", err)
			continue
		}
		for i := range pixes {
			pixes[i].ContaID = conta.ID
		}
		novos, err := s.Recebimentos.Registrar(ctx, pixes)
		if err != nil {
			r.Falhas++
			slog.ErrorContext(ctx, "conciliação: gravação falhou", "conta", conta.ID, "erro", err)
			continue
		}
		if novos > 0 {
			slog.InfoContext(ctx, "conciliação recuperou Pix sem aviso", "conta", conta.ID, "novos", novos)
		}
		r.Novos += novos
	}
	return r, nil
}

// conciliarRecorrencias consulta no PSP cada recorrência aberta da conta e
// aplica o que mudou pelo mesmo caminho do webhook de recorrência. Conta cujo
// PSP não faz Pix Automático é pulada sem falha.
func (s Conciliacao) conciliarRecorrencias(ctx context.Context, conta domain.Conta) (mudaram, falhas int) {
	if s.Recorrencias.Store == nil {
		return 0, 0
	}
	psp, err := pspRecorrencia(s.PSPs, conta)
	if err != nil {
		return 0, 0
	}
	abertas, err := s.Store.Recorrencias().Abertas(ctx, conta.ID)
	if err != nil {
		slog.ErrorContext(ctx, "conciliação: leitura das recorrências falhou", "conta", conta.ID, "erro", err)
		return 0, 1
	}
	for _, rec := range abertas {
		noBanco, err := psp.ConsultarRecorrencia(ctx, conta, rec.ID)
		if err != nil {
			falhas++
			slog.WarnContext(ctx, "conciliação: consulta da recorrência falhou", "conta", conta.ID, "idRec", rec.ID, "erro", err)
			continue
		}
		if noBanco.Status == rec.Status {
			continue
		}
		mudou, err := s.Recorrencias.RegistrarAtualizacao(ctx, conta.ID, domain.AtualizacaoRecorrencia{
			IDRec: rec.ID, Status: noBanco.Status, MotivoRejeicao: noBanco.MotivoRejeicao,
		})
		if err != nil {
			falhas++
			slog.ErrorContext(ctx, "conciliação: gravação da recorrência falhou", "conta", conta.ID, "idRec", rec.ID, "erro", err)
			continue
		}
		if mudou {
			mudaram++
			slog.InfoContext(ctx, "conciliação recuperou mudança de recorrência sem aviso", "conta", conta.ID, "idRec", rec.ID, "status", noBanco.Status)
		}
	}
	return mudaram, falhas
}

// conciliarCobrs consulta no PSP cada cobrança recorrente aberta da conta e
// aplica o que mudou pelo mesmo caminho do webhook de cobrança recorrente.
// Conta cujo PSP não faz cobrança recorrente é pulada sem falha.
func (s Conciliacao) conciliarCobrs(ctx context.Context, conta domain.Conta) (mudaram, falhas int) {
	if s.CobrancasRecorrentes.Store == nil {
		return 0, 0
	}
	if _, err := pspCobr(s.PSPs, conta); err != nil {
		return 0, 0
	}
	abertas, err := s.Store.CobrancasRecorrentes().Abertas(ctx, conta.ID)
	if err != nil {
		slog.ErrorContext(ctx, "conciliação: leitura das cobranças recorrentes falhou", "conta", conta.ID, "erro", err)
		return 0, 1
	}
	for _, c := range abertas {
		mudou, err := s.CobrancasRecorrentes.sincronizar(ctx, c)
		if err != nil {
			falhas++
			slog.WarnContext(ctx, "conciliação: cobrança recorrente não conferida", "conta", conta.ID, "txid", c.Txid, "erro", err)
			continue
		}
		if mudou {
			mudaram++
			slog.InfoContext(ctx, "conciliação recuperou mudança de cobrança recorrente sem aviso", "conta", conta.ID, "txid", c.Txid)
		}
	}
	return mudaram, falhas
}
