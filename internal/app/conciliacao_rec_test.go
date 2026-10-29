package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
)

func TestConciliacaoRecuperaMudancaDeRecorrenciaSemAviso(t *testing.T) {
	svc, s := comSolicitacao(t)
	banco := pspDaLoja(t, svc)
	conc := app.Conciliacao{Store: s, PSPs: svc.PSPs, Recebimentos: app.Recebimentos{Store: s, Clock: svc.Clock}, Recorrencias: svc, Clock: svc.Clock}
	ctx := context.Background()

	// O banco ainda não mudou: nada a fazer.
	banco.noBanco = domain.RecorrenciaCriada
	if r, err := conc.Rodar(ctx); err != nil || r.Recorrencias != 0 || r.Falhas != 0 {
		t.Fatalf("sem mudança: %+v, %v", r, err)
	}
	// O banco aprovou e o aviso se perdeu.
	banco.noBanco = domain.RecorrenciaAprovada
	r, err := conc.Rodar(ctx)
	if err != nil || r.Recorrencias != 1 || r.Falhas != 0 {
		t.Fatalf("aprovada sem aviso: %+v, %v", r, err)
	}
	rec, _ := s.Recorrencias().Buscar(ctx, idRecPSP)
	sol, _ := s.Solicitacoes().Buscar(ctx, idSolicRecPSP)
	if rec.Status != domain.RecorrenciaAprovada || sol.Status != domain.SolicitacaoAceita {
		t.Fatalf("rec %s, solicitação %s", rec.Status, sol.Status)
	}
	if evs := eventos(t, s, domain.EventoRecorrenciaAtualizada); len(evs) != 1 {
		t.Fatalf("eventos = %d; quer 1", len(evs))
	}
	// A próxima rodada não repete.
	if r, _ := conc.Rodar(ctx); r.Recorrencias != 0 {
		t.Fatalf("segunda rodada: %+v", r)
	}
}

func TestConciliacaoDeRecorrenciaComFalhaNaConsulta(t *testing.T) {
	svc, s := comSolicitacao(t)
	banco := pspDaLoja(t, svc)
	banco.errConsulta = errors.New("banco fora do ar")
	conc := app.Conciliacao{Store: s, PSPs: svc.PSPs, Recebimentos: app.Recebimentos{Store: s, Clock: svc.Clock}, Recorrencias: svc, Clock: svc.Clock}
	r, err := conc.Rodar(context.Background())
	if err != nil || r.Falhas != 1 || r.Recorrencias != 0 {
		t.Fatalf("consulta falhou: %+v, %v", r, err)
	}
	// Sem o caso de uso de recorrências, a conciliação nem consulta.
	conc.Recorrencias = app.Recorrencias{}
	if r, _ := conc.Rodar(context.Background()); r.Falhas != 0 {
		t.Fatalf("sem recorrências configuradas: %+v", r)
	}
}
