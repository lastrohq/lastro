package app_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

var agndAgendada = domain.Tentativa{DataLiquidacao: domain.NovaData(2026, 11, 5), Tipo: domain.TipoAgendamento, Status: domain.TentativaAgendada}

func ativa() domain.AtualizacaoCobrancaRecorrente {
	return domain.AtualizacaoCobrancaRecorrente{Txid: txidCobr, Status: domain.CobrAtiva, Tentativas: []domain.Tentativa{agndAgendada}}
}

func TestRegistrarAtualizacaoCobr(t *testing.T) {
	ctx := context.Background()
	svc, s, _, _ := novasCobrs(t)
	if _, err := svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr()); err != nil {
		t.Fatal(err)
	}
	mudou, err := svc.RegistrarAtualizacao(ctx, "loja", ativa())
	if err != nil || !mudou {
		t.Fatalf("ATIVA: %v, %v", mudou, err)
	}
	c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr)
	if c.Status != domain.CobrAtiva || len(c.Tentativas) != 1 || c.Revisao != 1 {
		t.Fatalf("gravada = %+v", c)
	}
	if evs := dadosCobr(t, s, domain.EventoCobrRAtualizada); len(evs) != 1 || evs[0].Status != "ATIVA" {
		t.Fatalf("atualizada = %+v", evs)
	}
	if evs := dadosCobr(t, s, domain.EventoCobrRTentativa); len(evs) != 1 || evs[0].Tentativa.Status != "AGENDADA" {
		t.Fatalf("tentativa = %+v", evs)
	}

	// Aviso repetido: nada muda, nada é publicado.
	if mudou, err := svc.RegistrarAtualizacao(ctx, "loja", ativa()); err != nil || mudou {
		t.Fatalf("repetido: %v, %v", mudou, err)
	}
	// A tentativa falha: um evento de tentativa, sem atualizada.
	falhou := ativa()
	falhou.Tentativas[0].Status, falhou.Tentativas[0].MotivoRejeicao = domain.TentativaRejeitada, "AM04: saldo insuficiente"
	if mudou, err := svc.RegistrarAtualizacao(ctx, "loja", falhou); err != nil || !mudou {
		t.Fatalf("tentativa rejeitada: %v, %v", mudou, err)
	}
	if n := len(dadosCobr(t, s, domain.EventoCobrRTentativa)); n != 2 {
		t.Fatalf("eventos de tentativa = %d", n)
	}
	if n := len(dadosCobr(t, s, domain.EventoCobrRAtualizada)); n != 1 {
		t.Fatalf("eventos de atualizada = %d", n)
	}

	// Paga, e depois um aviso velho que voltaria para ATIVA: ignorado.
	paga := ativa()
	paga.Status, paga.Tentativas = domain.CobrConcluida, nil
	_, _ = svc.RegistrarAtualizacao(ctx, "loja", paga)
	if mudou, err := svc.RegistrarAtualizacao(ctx, "loja", ativa()); err != nil || mudou {
		t.Fatalf("regressão: %v, %v", mudou, err)
	}
	if c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr); c.Status != domain.CobrConcluida {
		t.Fatalf("status depois da regressão: %s", c.Status)
	}

	for conta, txid := range map[string]domain.Txid{"outra": txidCobr, "loja": "cobr0000000000000000000099999"} {
		a := ativa()
		a.Txid = txid
		if _, err := svc.RegistrarAtualizacao(ctx, conta, a); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("%s/%s: %v", conta, txid, err)
		}
	}
}

func TestConsultarCobrSincronizaComOBanco(t *testing.T) {
	ctx := context.Background()
	svc, s, p, _ := novasCobrs(t)
	_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
	p.noBanco = &domain.CobrancaRecorrente{Txid: txidCobr, Status: domain.CobrAtiva, Tentativas: []domain.Tentativa{agndAgendada}}
	c, err := svc.Consultar(ctx, txidCobr)
	if err != nil || c.Status != domain.CobrAtiva || len(c.Tentativas) != 1 {
		t.Fatalf("Consultar = %+v, %v", c, err)
	}
	if evs := dadosCobr(t, s, domain.EventoCobrRAtualizada); len(evs) != 1 {
		t.Fatalf("eventos = %+v", evs)
	}

	// PSP fora do ar: devolve o que está gravado.
	p.errConsulta = errors.New("banco fora do ar")
	if c, err := svc.Consultar(ctx, txidCobr); err != nil || c.Status != domain.CobrAtiva {
		t.Fatalf("PSP fora do ar: %+v, %v", c, err)
	}

	// Terminal não consulta o banco: nem a falha chega.
	p.errConsulta = nil
	p.noBanco = &domain.CobrancaRecorrente{Txid: txidCobr, Status: domain.CobrConcluida}
	_, _ = svc.Consultar(ctx, txidCobr)
	p.noBanco = &domain.CobrancaRecorrente{Txid: txidCobr, Status: domain.CobrCancelada}
	if c, _ := svc.Consultar(ctx, txidCobr); c.Status != domain.CobrConcluida {
		t.Fatalf("concluída mudou para %s", c.Status)
	}
}

func webhooksCobr(svc app.CobrancasRecorrentes) app.Webhooks {
	return app.Webhooks{Store: svc.Store, PSPs: svc.PSPs, Recebimentos: app.Recebimentos{Store: svc.Store, Clock: svc.Clock}, CobrancasRecorrentes: svc}
}

func TestWebhookCobr(t *testing.T) {
	ctx := context.Background()
	svc, s, p, _ := novasCobrs(t)
	_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
	wh := webhooksCobr(svc)
	endereco, err := wh.Configurar(ctx, "loja", "https://lastro.example.com")
	if err != nil {
		t.Fatal(err)
	}
	token := endereco[strings.LastIndex(endereco, "/")+1:]
	if p.webhookCobr != "https://lastro.example.com/webhooks/cobr/loja/"+token {
		t.Fatalf("webhookcobr = %q", p.webhookCobr)
	}

	corpo := `{"cobsr":[{"idRec":"` + idRecDaConta["loja"] + `","txid":"` + string(txidCobr) + `","status":"ATIVA",
	 "tentativas":[{"dataLiquidacao":"2026-11-05","tipo":"AGND","status":"AGENDADA","endToEndId":""}]},
	 {"idRec":"` + idRecDaConta["loja"] + `","txid":"cobr0000000000000000000099999","status":"ATIVA"}]}`
	if _, err := wh.ReceberCobr(ctx, "loja", strings.Repeat("0", 48), []byte(corpo)); !errors.Is(err, domain.ErrNaoAutenticado) {
		t.Fatalf("token errado: %v", err)
	}
	if _, err := wh.ReceberCobr(ctx, "loja", token, []byte(`{"cobsr":[]}`)); !errors.Is(err, domain.ErrCobrancaRecorrenteInvalida) {
		t.Fatalf("corpo inválido: %v", err)
	}
	// A cobrança desconhecida é ignorada; a conhecida muda.
	n, err := wh.ReceberCobr(ctx, "loja", token, []byte(corpo))
	if err != nil || n != 1 {
		t.Fatalf("ReceberCobr = %d, %v", n, err)
	}
	if c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr); c.Status != domain.CobrAtiva || len(c.Tentativas) != 1 {
		t.Fatalf("gravada = %+v", c)
	}
}

func TestConciliacaoDeCobrancasRecorrentes(t *testing.T) {
	ctx := context.Background()
	svc, _, p, _ := novasCobrs(t)
	_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
	conc := app.Conciliacao{Store: svc.Store, PSPs: svc.PSPs, Recebimentos: app.Recebimentos{Store: svc.Store, Clock: svc.Clock}, CobrancasRecorrentes: svc, Clock: svc.Clock}

	p.noBanco = &domain.CobrancaRecorrente{Txid: txidCobr, Status: domain.CobrAtiva}
	if r, err := conc.Rodar(ctx); err != nil || r.CobrancasRecorrentes != 1 {
		t.Fatalf("primeira rodada: %+v, %v", r, err)
	}
	if r, _ := conc.Rodar(ctx); r.CobrancasRecorrentes != 0 {
		t.Fatalf("segunda rodada mudou de novo: %+v", r)
	}
	p.errConsulta = errors.New("banco fora do ar")
	if r, _ := conc.Rodar(ctx); r.Falhas == 0 {
		t.Fatalf("falha não contada: %+v", r)
	}
	// Sem CobrancasRecorrentes configurada, a conciliação não olha as cobranças.
	conc.CobrancasRecorrentes = app.CobrancasRecorrentes{}
	p.errConsulta, p.noBanco = nil, &domain.CobrancaRecorrente{Txid: txidCobr, Status: domain.CobrConcluida}
	if r, _ := conc.Rodar(ctx); r.CobrancasRecorrentes != 0 {
		t.Fatalf("conciliou sem estar configurada: %+v", r)
	}
}
