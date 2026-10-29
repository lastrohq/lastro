package sandbox_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

func recSandbox(politica domain.PoliticaRetentativa) domain.Recorrencia {
	return domain.Recorrencia{Contrato: "1", Devedor: domain.Devedor{Documento: "12345678909", Nome: "F"}, DataInicial: domain.NovaData(2026, 11, 10), Periodicidade: domain.Mensal, ValorFixo: 100, Politica: politica}
}

func TestRecorrenciaNoSandbox(t *testing.T) {
	p := sandbox.New(fake.NewClock(inicio))
	ctx := context.Background()
	com, err := p.CriarRecorrencia(ctx, conta, recSandbox(domain.Retentativa3Em7Dias))
	if err != nil {
		t.Fatal(err)
	}
	sem, _ := p.CriarRecorrencia(ctx, conta, recSandbox(domain.SemRetentativa))
	for _, r := range []domain.Recorrencia{com, sem} {
		if err := domain.ValidarIDRecorrencia(r.ID); err != nil || r.Status != domain.RecorrenciaCriada || !strings.Contains(r.ID, "0000000020261029") {
			t.Fatalf("idRec %q, status %s: %v", r.ID, r.Status, err)
		}
	}
	// O segundo caractere diz se a recorrência permite retentativa.
	if com.ID[1] != 'R' || sem.ID[1] != 'N' || com.ID == sem.ID {
		t.Fatalf("ids: %s %s", com.ID, sem.ID)
	}
	outra := conta
	outra.ID = "outra"
	if _, err := p.ConsultarRecorrencia(ctx, outra, com.ID); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("outra conta: %v", err)
	}
	sol, err := p.SolicitarConfirmacao(ctx, conta, domain.SolicitacaoRecorrencia{RecorrenciaID: com.ID, ExpiraEm: inicio.Add(time.Hour)})
	if err != nil || len(sol.ID) != 29 || !strings.HasPrefix(sol.ID, "SC00000000") || sol.Status != domain.SolicitacaoEnviada {
		t.Fatalf("solicitação = %+v, %v", sol, err)
	}
	if _, err := p.SolicitarConfirmacao(ctx, conta, domain.SolicitacaoRecorrencia{RecorrenciaID: "RR0000000020261029naoexiste1"}); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("solicitação de recorrência inexistente: %v", err)
	}
}

func TestRecorrenciaLembradaDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	ctx := context.Background()
	_ = store.Contas().Criar(ctx, conta)
	criada, _ := sandbox.New(clock, sandbox.ComStore(store)).CriarRecorrencia(ctx, conta, recSandbox(domain.SemRetentativa))
	if err := store.Recorrencias().Criar(ctx, criada); err != nil {
		t.Fatal(err)
	}
	novo := sandbox.New(clock, sandbox.ComStore(store))
	if r, err := novo.ConsultarRecorrencia(ctx, conta, criada.ID); err != nil || r.ID != criada.ID {
		t.Fatalf("depois do restart: %+v, %v", r, err)
	}
}
