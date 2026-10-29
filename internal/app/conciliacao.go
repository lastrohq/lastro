package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/lastrohq/lastro/internal/ports"
)

// JanelaConciliacaoPadrao é quanto tempo para trás a conciliação olha (RN-31).
const JanelaConciliacaoPadrao = 2 * time.Hour

// Conciliacao recupera os Pix que o banco recebeu mas não avisou (RN-31).
type Conciliacao struct {
	Store        ports.Store
	PSPs         ports.Registry
	Recebimentos Recebimentos
	Clock        ports.Clock
	// Janela é quanto tempo para trás consultar; zero usa JanelaConciliacaoPadrao.
	Janela time.Duration
}

// ResultadoConciliacao resume uma rodada.
type ResultadoConciliacao struct {
	Contas int
	// Novos são os Pix que não tinham chegado por webhook.
	Novos  int
	Falhas int
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
