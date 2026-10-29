package storetest

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

func recorrencia(id, conta string) domain.Recorrencia {
	return domain.Recorrencia{
		ID: id, ContaID: conta, Status: domain.RecorrenciaCriada, Contrato: "63100862", Objeto: "Academia",
		Devedor:     domain.Devedor{Documento: "12345678909", Nome: "Fulano"},
		DataInicial: domain.NovaData(2026, 11, 10), DataFinal: domain.NovaData(2027, 11, 10), Periodicidade: domain.Mensal,
		ValorFixo: 9990, Politica: domain.Retentativa3Em7Dias, CriadaEm: base,
	}
}

// runRecorrencias é a parte da suíte para o Pix Automático.
func runRecorrencias(t *testing.T, novo func(t *testing.T) ports.Store) {
	t.Run("recorrência: cria, busca, recusa duplicada e conta inexistente", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		r := recorrencia("RR1234567820261029abcDEF00001", "c1")
		semFim := recorrencia("RN1234567820261029abcDEF00002", "c1")
		semFim.DataFinal, semFim.ValorFixo, semFim.PisoValorMaximo, semFim.Politica = domain.Data{}, 0, 5000, domain.SemRetentativa
		for _, rec := range []domain.Recorrencia{r, semFim} {
			if err := s.Recorrencias().Criar(ctx, rec); err != nil {
				t.Fatal(err)
			}
			got, err := s.Recorrencias().Buscar(ctx, rec.ID)
			if err != nil || !reflect.DeepEqual(got, rec) {
				t.Fatalf("Buscar = %+v, %v\nquer   %+v", got, err, rec)
			}
		}
		if err := s.Recorrencias().Criar(ctx, r); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("duplicada: %v", err)
		}
		if err := s.Recorrencias().Criar(ctx, recorrencia("RR1234567820261029abcDEF00003", "nada")); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("conta inexistente: %v", err)
		}
		if _, err := s.Recorrencias().Buscar(ctx, "RR1234567820261029abcDEF99999"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("recorrência: atualizar segue a revisão", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		r := recorrencia("RR1234567820261029abcDEF00004", "c1")
		_ = s.Recorrencias().Criar(ctx, r)
		velha := r
		r.Status, r.MotivoRejeicao = domain.RecorrenciaRejeitada, "AP13"
		if err := s.Recorrencias().Atualizar(ctx, &r); err != nil || r.Revisao != 1 {
			t.Fatalf("Atualizar: revisão %d, %v", r.Revisao, err)
		}
		got, _ := s.Recorrencias().Buscar(ctx, r.ID)
		if got.Status != domain.RecorrenciaRejeitada || got.MotivoRejeicao != "AP13" || got.Revisao != 1 {
			t.Fatalf("depois de atualizar: %+v", got)
		}
		if err := s.Recorrencias().Atualizar(ctx, &velha); !errors.Is(err, ports.ErrConflito) {
			t.Fatalf("revisão velha: %v", err)
		}
		fantasma := recorrencia("RR1234567820261029abcDEF99999", "c1")
		if err := s.Recorrencias().Atualizar(ctx, &fantasma); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
	})

	t.Run("recorrência: abertas são as CRIADA e APROVADA da conta", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		ids := map[domain.StatusRecorrencia]string{
			domain.RecorrenciaCriada: "RR1234567820261029abcDEF00011", domain.RecorrenciaAprovada: "RR1234567820261029abcDEF00012",
			domain.RecorrenciaRejeitada: "RR1234567820261029abcDEF00013", domain.RecorrenciaCancelada: "RR1234567820261029abcDEF00014",
		}
		i := 0
		for status, id := range ids {
			r := recorrencia(id, "c1")
			r.Status, r.CriadaEm = status, base.Add(time.Duration(i)*time.Minute)
			i++
			if err := s.Recorrencias().Criar(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
		daOutra := recorrencia("RR1234567820261029abcDEF00015", "c2")
		_ = s.Recorrencias().Criar(ctx, daOutra)
		abertas, err := s.Recorrencias().Abertas(ctx, "c1")
		if err != nil || len(abertas) != 2 {
			t.Fatalf("Abertas = %+v, %v", abertas, err)
		}
		for _, r := range abertas {
			if r.Status != domain.RecorrenciaCriada && r.Status != domain.RecorrenciaAprovada {
				t.Fatalf("aberta com status %s", r.Status)
			}
		}
		if abertas[0].CriadaEm.After(abertas[1].CriadaEm) {
			t.Fatal("fora da ordem de criação")
		}
	})

	t.Run("solicitação: cria, busca, atualiza o status e exige a recorrência", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		r := recorrencia("RR1234567820261029abcDEF00005", "c1")
		_ = s.Recorrencias().Criar(ctx, r)
		sol := domain.SolicitacaoRecorrencia{
			ID: "SC1234567820261029abcDEF00001", RecorrenciaID: r.ID, ContaID: "c1", Status: domain.SolicitacaoCriada,
			Destinatario: domain.ContaPagador{ISPB: "18236120", Agencia: "0001", Conta: "123456"},
			ExpiraEm:     base.Add(7 * 24 * time.Hour), CriadaEm: base,
		}
		if err := s.Solicitacoes().Criar(ctx, sol); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Solicitacoes().Buscar(ctx, sol.ID); err != nil || !reflect.DeepEqual(got, sol) {
			t.Fatalf("Buscar = %+v, %v", got, err)
		}
		if err := s.Solicitacoes().Criar(ctx, sol); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("duplicada: %v", err)
		}
		orfa := sol
		orfa.ID, orfa.RecorrenciaID = "SC1234567820261029abcDEF00002", "RR1234567820261029abcDEF99999"
		if err := s.Solicitacoes().Criar(ctx, orfa); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("recorrência inexistente: %v", err)
		}
		if err := s.Solicitacoes().AtualizarStatus(ctx, sol.ID, domain.SolicitacaoEnviada); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Solicitacoes().Buscar(ctx, sol.ID); got.Status != domain.SolicitacaoEnviada {
			t.Fatalf("status = %s", got.Status)
		}
		if err := s.Solicitacoes().AtualizarStatus(ctx, "SC1234567820261029abcDEF99999", domain.SolicitacaoEnviada); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: %v", err)
		}
		segunda := sol
		segunda.ID, segunda.CriadaEm = "SC1234567820261029abcDEF00003", base.Add(time.Hour)
		if err := s.Solicitacoes().Criar(ctx, segunda); err != nil {
			t.Fatal(err)
		}
		lista, err := s.Solicitacoes().DaRecorrencia(ctx, r.ID)
		if err != nil || len(lista) != 2 || lista[0].ID != sol.ID || lista[1].ID != segunda.ID {
			t.Fatalf("DaRecorrencia = %+v, %v", lista, err)
		}
		if vazia, err := s.Solicitacoes().DaRecorrencia(ctx, "RR1234567820261029abcDEF99999"); err != nil || len(vazia) != 0 {
			t.Fatalf("de recorrência sem solicitação: %+v, %v", vazia, err)
		}
	})
}
