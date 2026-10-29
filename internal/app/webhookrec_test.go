package app_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

func avisoRec(status, extra string) []byte {
	return []byte(`{"recs":[{"idRec":"` + idRecPSP + `","status":"` + status + `","atualizacao":[]` + extra + `}]}`)
}

// comSolicitacao cria a recorrência e a solicitação, como a jornada 1 faz.
func comSolicitacao(t *testing.T) (app.Recorrencias, *fake.Store) {
	t.Helper()
	svc, s, _ := novasRecorrencias(t)
	ctx := context.Background()
	if _, err := svc.Criar(ctx, "loja", pedidoRec()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SolicitarConfirmacao(ctx, idRecPSP, domain.ContaPagador{ISPB: "18236120", Conta: "1"}, agoraRec.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return svc, s
}

func TestAprovacaoMoveRecorrenciaESolicitacao(t *testing.T) {
	svc, s := comSolicitacao(t)
	ctx := context.Background()
	mudou, err := svc.RegistrarAtualizacao(ctx, "loja", domain.AtualizacaoRecorrencia{IDRec: idRecPSP, Status: domain.RecorrenciaAprovada})
	if err != nil || !mudou {
		t.Fatalf("aprovar: %v, %v", mudou, err)
	}
	rec, _ := s.Recorrencias().Buscar(ctx, idRecPSP)
	sol, _ := s.Solicitacoes().Buscar(ctx, idSolicRecPSP)
	if rec.Status != domain.RecorrenciaAprovada || rec.Revisao != 1 || sol.Status != domain.SolicitacaoAceita {
		t.Fatalf("rec %s (revisão %d), solicitação %s", rec.Status, rec.Revisao, sol.Status)
	}
	if evs := eventos(t, s, domain.EventoRecorrenciaAtualizada); len(evs) != 1 || !strings.Contains(string(evs[0]), `"status":"APROVADA"`) {
		t.Fatalf("eventos = %s", evs)
	}
	// O mesmo aviso de novo não muda nada nem publica outro evento.
	if mudou, err := svc.RegistrarAtualizacao(ctx, "loja", domain.AtualizacaoRecorrencia{IDRec: idRecPSP, Status: domain.RecorrenciaAprovada}); err != nil || mudou {
		t.Fatalf("repetido: %v, %v", mudou, err)
	}
	// Aviso fora de ordem (APROVADA → CRIADA) é ignorado.
	if mudou, err := svc.RegistrarAtualizacao(ctx, "loja", domain.AtualizacaoRecorrencia{IDRec: idRecPSP, Status: domain.RecorrenciaCriada}); err != nil || mudou {
		t.Fatalf("regressão: %v, %v", mudou, err)
	}
	if evs := eventos(t, s, domain.EventoRecorrenciaAtualizada); len(evs) != 1 {
		t.Fatalf("eventos depois do repetido e da regressão = %d; quer 1", len(evs))
	}
}

func TestRejeicaoEExpiracao(t *testing.T) {
	for _, c := range []struct {
		status domain.StatusRecorrencia
		sol    domain.StatusSolicitacao
		motivo string
	}{
		{domain.RecorrenciaRejeitada, domain.SolicitacaoRejeitada, "AP13: Pagador não reconhece o recebedor"},
		{domain.RecorrenciaExpirada, domain.SolicitacaoExpirada, ""},
	} {
		t.Run(string(c.status), func(t *testing.T) {
			svc, s := comSolicitacao(t)
			ctx := context.Background()
			if _, err := svc.RegistrarAtualizacao(ctx, "loja", domain.AtualizacaoRecorrencia{IDRec: idRecPSP, Status: c.status, MotivoRejeicao: c.motivo}); err != nil {
				t.Fatal(err)
			}
			rec, _ := s.Recorrencias().Buscar(ctx, idRecPSP)
			sol, _ := s.Solicitacoes().Buscar(ctx, idSolicRecPSP)
			if rec.Status != c.status || rec.MotivoRejeicao != c.motivo || sol.Status != c.sol {
				t.Fatalf("rec %s motivo %q, solicitação %s", rec.Status, rec.MotivoRejeicao, sol.Status)
			}
		})
	}
}

func TestRecorrenciaDeOutraContaNoAviso(t *testing.T) {
	svc, _ := comSolicitacao(t)
	if _, err := svc.RegistrarAtualizacao(context.Background(), "outra", domain.AtualizacaoRecorrencia{IDRec: idRecPSP, Status: domain.RecorrenciaAprovada}); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("err = %v", err)
	}
}

func TestReceberRecConfereTokenEAplica(t *testing.T) {
	svc, s := comSolicitacao(t)
	ctx := context.Background()
	h := sha256.Sum256([]byte("token-certo"))
	if err := s.Contas().SalvarTokenWebhook(ctx, "loja", h[:]); err != nil {
		t.Fatal(err)
	}
	wh := app.Webhooks{Store: s, PSPs: svc.PSPs, Recorrencias: svc}

	if _, err := wh.ReceberRec(ctx, "loja", "token-errado", avisoRec("APROVADA", "")); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("token errado: %v", err)
	}
	if _, err := wh.ReceberRec(ctx, "loja", "token-certo", []byte(`{"recs":[]}`)); !errors.Is(err, domain.ErrRecorrenciaInvalida) {
		t.Fatalf("corpo inválido: %v", err)
	}
	n, err := wh.ReceberRec(ctx, "loja", "token-certo", avisoRec("REJEITADA", `,"encerramento":{"rejeicao":{"codigo":"AP14","descricao":"x"}}`))
	if err != nil || n != 1 {
		t.Fatalf("receber: %d, %v", n, err)
	}
	if rec, _ := s.Recorrencias().Buscar(ctx, idRecPSP); rec.Status != domain.RecorrenciaRejeitada || rec.MotivoRejeicao != "AP14: x" {
		t.Fatalf("rec = %s %q", rec.Status, rec.MotivoRejeicao)
	}
	// Recorrência que o Lastro não conhece: ignorada, sem fazer o banco reenviar.
	desconhecida := []byte(`{"recs":[{"idRec":"RR9999999920261029abcDEF00009","status":"APROVADA","atualizacao":[]}]}`)
	if n, err := wh.ReceberRec(ctx, "loja", "token-certo", desconhecida); err != nil || n != 0 {
		t.Fatalf("desconhecida: %d, %v", n, err)
	}
}

func TestConfigurarRegistraOWebhookDeRecorrencia(t *testing.T) {
	svc, s, p := novasRecorrencias(t)
	wh := app.Webhooks{Store: s, PSPs: svc.PSPs, Recorrencias: svc}
	p.caps[ports.CapWebhook] = true
	endereco, err := wh.Configurar(context.Background(), "loja", "https://lastro.example.com")
	if err != nil {
		t.Fatal(err)
	}
	token := endereco[strings.LastIndex(endereco, "/")+1:]
	if p.webhookRec != "https://lastro.example.com/webhooks/rec/loja/"+token {
		t.Fatalf("webhookrec = %q (pix em %q)", p.webhookRec, endereco)
	}
}
