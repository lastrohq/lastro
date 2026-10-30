package sandbox_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
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

func TestRespostaDoPagadorPeloHorarioDaSolicitacao(t *testing.T) {
	for valor, quer := range map[domain.Money]domain.StatusRecorrencia{
		100: domain.RecorrenciaAprovada, sandbox.ValorRecRejeitada: domain.RecorrenciaRejeitada, sandbox.ValorRecExpirada: domain.RecorrenciaExpirada,
	} {
		t.Run(string(quer), func(t *testing.T) {
			clock := fake.NewClock(inicio)
			p := sandbox.New(clock)
			ctx := context.Background()
			r := recSandbox(domain.SemRetentativa)
			r.ValorFixo = valor
			criada, _ := p.CriarRecorrencia(ctx, conta, r)
			// Antes de pedir a confirmação, nada muda, por mais tempo que passe.
			clock.Avancar(time.Hour)
			if lida, _ := p.ConsultarRecorrencia(ctx, conta, criada.ID); lida.Status != domain.RecorrenciaCriada {
				t.Fatalf("sem solicitação: %s", lida.Status)
			}
			if _, err := p.SolicitarConfirmacao(ctx, conta, domain.SolicitacaoRecorrencia{RecorrenciaID: criada.ID}); err != nil {
				t.Fatal(err)
			}
			clock.Avancar(sandbox.AtrasoPadrao - time.Second)
			if lida, _ := p.ConsultarRecorrencia(ctx, conta, criada.ID); lida.Status != domain.RecorrenciaCriada {
				t.Fatalf("antes do atraso: %s", lida.Status)
			}
			clock.Avancar(time.Second)
			lida, _ := p.ConsultarRecorrencia(ctx, conta, criada.ID)
			if lida.Status != quer || (quer == domain.RecorrenciaRejeitada) != (lida.MotivoRejeicao == "AP13") {
				t.Fatalf("depois do atraso: %s %q; quer %s", lida.Status, lida.MotivoRejeicao, quer)
			}
		})
	}
}

func TestRespostaDoPagadorDepoisDoRestart(t *testing.T) {
	store, clock := fake.NewStore(), fake.NewClock(inicio)
	ctx := context.Background()
	_ = store.Contas().Criar(ctx, conta)
	antigo := sandbox.New(clock, sandbox.ComStore(store))
	criada, _ := antigo.CriarRecorrencia(ctx, conta, recSandbox(domain.SemRetentativa))
	_ = store.Recorrencias().Criar(ctx, criada)
	sol, _ := antigo.SolicitarConfirmacao(ctx, conta, domain.SolicitacaoRecorrencia{RecorrenciaID: criada.ID, ExpiraEm: inicio.Add(time.Hour)})
	if err := store.Solicitacoes().Criar(ctx, sol); err != nil {
		t.Fatal(err)
	}
	clock.Avancar(sandbox.AtrasoPadrao)
	novo := sandbox.New(clock, sandbox.ComStore(store)) // outro processo: memória vazia
	if lida, err := novo.ConsultarRecorrencia(ctx, conta, criada.ID); err != nil || lida.Status != domain.RecorrenciaAprovada {
		t.Fatalf("depois do restart: %s, %v", lida.Status, err)
	}
}
