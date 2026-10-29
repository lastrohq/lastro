package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// pspDevolucao é um PSP falso que devolve.
type pspDevolucao struct {
	ports.PSP
	mu       sync.Mutex
	err      error
	chamadas int
	// status é o que Devolver responde; vazio responde sem status.
	status domain.StatusDevolucao
	// outroValor faz Devolver responder com um valor diferente do pedido.
	outroValor domain.Money
	// pedidos guarda o que chegou ao banco.
	pedidos []domain.Devolucao
	// espera segura a resposta, para pedidos simultâneos se cruzarem.
	espera time.Duration
}

func (p *pspDevolucao) Nome() string { return "banco-dev" }
func (p *pspDevolucao) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCob: true, ports.CapDevolucao: true}
}

func (p *pspDevolucao) Devolver(_ context.Context, _ domain.Conta, d domain.Devolucao) (domain.Devolucao, error) {
	time.Sleep(p.espera)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.chamadas++
	p.pedidos = append(p.pedidos, d)
	if p.err != nil {
		return domain.Devolucao{}, p.err
	}
	d.Status, d.RtrID = p.status, "D18236120202610291200rtr0000001"
	if p.outroValor != 0 {
		d.Valor = p.outroValor
	}
	return d, nil
}

// semDevolucao faz cob mas não devolve.
type semDevolucao struct{ ports.PSP }

func (semDevolucao) Nome() string                     { return "sem-dev" }
func (semDevolucao) Capabilities() ports.Capabilities { return ports.Capabilities{ports.CapCob: true} }

// pagoEm é quando os Pix do teste entraram; agoraDev, um dia depois.
var (
	pagoEm   = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	agoraDev = pagoEm.Add(24 * time.Hour)
)

const (
	e2eLoja    = "E18236120202610281200loja0000001"
	e2eOutra   = "E18236120202610281200outra000001"
	e2eSemDev  = "E18236120202610281200semdev00001"
	e2eAntigo  = "E18236120202607301200antigo00001"
	valorDoPix = domain.Money(10000)
)

func novasDevolucoes(t *testing.T) (app.Devolucoes, *fake.Store, *pspDevolucao, *fake.Clock) {
	t.Helper()
	s, ctx := fake.NewStore(), context.Background()
	p := &pspDevolucao{status: domain.DevolucaoEmProcessamento}
	contas := map[string]string{"loja": "banco-dev", "outra": "banco-dev", "sem-dev": "sem-dev"}
	for conta, nomePSP := range contas {
		if err := s.Contas().Criar(ctx, domain.Conta{ID: conta, Nome: conta, PSP: nomePSP, ChavePix: conta + "@example.com", Ambiente: domain.Producao, Ativa: true}); err != nil {
			t.Fatal(err)
		}
	}
	pixes := []domain.PixRecebido{
		{EndToEndID: e2eLoja, ContaID: "loja", Valor: valorDoPix, Horario: pagoEm},
		{EndToEndID: e2eOutra, ContaID: "outra", Valor: valorDoPix, Horario: pagoEm},
		{EndToEndID: e2eSemDev, ContaID: "sem-dev", Valor: valorDoPix, Horario: pagoEm},
		{EndToEndID: e2eAntigo, ContaID: "loja", Valor: valorDoPix, Horario: agoraDev.Add(-domain.PrazoDevolucao - time.Minute)},
	}
	for _, px := range pixes {
		px.Txid, px.Chave = "txid00000000000000000000001", px.ContaID+"@example.com"
		if _, err := s.Pix().Registrar(ctx, px); err != nil {
			t.Fatal(err)
		}
	}
	clock := fake.NewClock(agoraDev)
	return app.Devolucoes{Store: s, PSPs: psp.NewRegistry(p, semDevolucao{}), Clock: clock}, s, p, clock
}

func pedidoDev(id string, valor domain.Money) domain.Devolucao {
	return domain.Devolucao{EndToEndID: e2eLoja, ID: id, Valor: valor, Descricao: "Produto devolvido"}
}

func dadosDev(t *testing.T, s *fake.Store, tipo domain.TipoEvento) []app.DadosDevolucao {
	t.Helper()
	var out []app.DadosDevolucao
	for _, raw := range eventos(t, s, tipo) {
		var d app.DadosDevolucao
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func TestSolicitarDevolucaoGravaEPublica(t *testing.T) {
	svc, s, p, _ := novasDevolucoes(t)
	ctx := context.Background()
	d, err := svc.Solicitar(ctx, pedidoDev("dev1", 4000))
	if err != nil {
		t.Fatal(err)
	}
	if d.ContaID != "loja" || d.Status != domain.DevolucaoEmProcessamento || d.Natureza != domain.DevolucaoOriginal ||
		d.RtrID == "" || !d.CriadaEm.Equal(agoraDev) || p.chamadas != 1 {
		t.Fatalf("devolução = %+v, chamadas = %d", d, p.chamadas)
	}
	if p.pedidos[0].Natureza != domain.DevolucaoOriginal || p.pedidos[0].Descricao != "Produto devolvido" {
		t.Fatalf("pedido ao banco = %+v", p.pedidos[0])
	}
	gravada, err := s.Devolucoes().Buscar(ctx, e2eLoja, "dev1")
	if err != nil || gravada != d {
		t.Fatalf("gravada = %+v, %v\nquer      %+v", gravada, err, d)
	}
	ev := dadosDev(t, s, domain.EventoDevolucaoSolicitada)
	if len(ev) != 1 || ev[0].ID != "dev1" || ev[0].EndToEndID != e2eLoja || ev[0].Valor != "40.00" || ev[0].Status != "EM_PROCESSAMENTO" || ev[0].RtrID != d.RtrID {
		t.Fatalf("eventos = %+v", ev)
	}

	// O resto do saldo, numa segunda devolução, também passa.
	if _, err := svc.Solicitar(ctx, pedidoDev("dev2", 6000)); err != nil {
		t.Fatalf("o resto do saldo: %v", err)
	}
	if _, err := svc.Solicitar(ctx, pedidoDev("dev3", 1)); !errors.Is(err, domain.ErrDevolucaoInvalida) || p.chamadas != 2 {
		t.Fatalf("depois de devolver tudo: %v, chamadas = %d", err, p.chamadas)
	}
}

func TestSolicitarDevolucaoIdempotentePeloPar(t *testing.T) {
	svc, s, p, _ := novasDevolucoes(t)
	ctx := context.Background()
	primeira, err := svc.Solicitar(ctx, pedidoDev("dev1", 4000))
	if err != nil {
		t.Fatal(err)
	}
	repetida, err := svc.Solicitar(ctx, pedidoDev("dev1", 4000))
	if err != nil || repetida != primeira || p.chamadas != 1 || len(dadosDev(t, s, domain.EventoDevolucaoSolicitada)) != 1 {
		t.Fatalf("repetida = %+v, %v, chamadas = %d", repetida, err, p.chamadas)
	}
	if _, err := svc.Solicitar(ctx, pedidoDev("dev1", 5000)); !errors.Is(err, ports.ErrJaExiste) || p.chamadas != 1 {
		t.Fatalf("mesmo par com outro valor: %v, chamadas = %d", err, p.chamadas)
	}
}

func TestSolicitarDevolucaoRecusaAntesDoBanco(t *testing.T) {
	casos := map[string]struct {
		pedido func() domain.Devolucao
		ctx    func() context.Context
		quer   error
	}{
		"passa do valor recebido": {pedido: func() domain.Devolucao { return pedidoDev("dev1", valorDoPix+1) }, quer: domain.ErrDevolucaoInvalida},
		"pix de mais de 90 dias": {pedido: func() domain.Devolucao {
			d := pedidoDev("dev1", 100)
			d.EndToEndID = e2eAntigo
			return d
		}, quer: domain.ErrDevolucaoInvalida},
		"id fora do formato":  {pedido: func() domain.Devolucao { return pedidoDev("dev-1", 100) }, quer: domain.ErrDevolucaoInvalida},
		"e2e fora do formato": {pedido: func() domain.Devolucao { d := pedidoDev("dev1", 100); d.EndToEndID = "curto"; return d }, quer: app.ErrRequisicaoInvalida},
		"pix desconhecido": {pedido: func() domain.Devolucao {
			d := pedidoDev("dev1", 100)
			d.EndToEndID = "E18236120202610281200nada0000001"
			return d
		}, quer: ports.ErrNaoEncontrado},
		"natureza retirada": {pedido: func() domain.Devolucao {
			d := pedidoDev("dev1", 100)
			d.Natureza = domain.DevolucaoRetirada
			return d
		}, quer: domain.ErrDevolucaoInvalida},
		"pix de outra conta": {
			pedido: func() domain.Devolucao { return pedidoDev("dev1", 100) },
			ctx:    func() context.Context { return app.ComAcesso(context.Background(), app.Acesso{ContaID: "outra"}) },
			quer:   ports.ErrNaoEncontrado,
		},
		"psp que não devolve": {pedido: func() domain.Devolucao { d := pedidoDev("dev1", 100); d.EndToEndID = e2eSemDev; return d }, quer: ports.ErrNaoSuportado},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			svc, s, p, _ := novasDevolucoes(t)
			ctx := context.Background()
			if c.ctx != nil {
				ctx = c.ctx()
			}
			if _, err := svc.Solicitar(ctx, c.pedido()); !errors.Is(err, c.quer) {
				t.Fatalf("err = %v, quer %v", err, c.quer)
			}
			if p.chamadas != 0 || len(s.Eventos()) != 0 {
				t.Fatalf("chamou o banco %d vezes e publicou %d eventos", p.chamadas, len(s.Eventos()))
			}
		})
	}
}

func TestSolicitarDevolucaoNaoRealizadaLiberaOSaldo(t *testing.T) {
	svc, _, p, _ := novasDevolucoes(t)
	ctx := context.Background()
	p.status = domain.DevolucaoNaoRealizado
	d, err := svc.Solicitar(ctx, pedidoDev("dev1", valorDoPix))
	if err != nil || d.Status != domain.DevolucaoNaoRealizado {
		t.Fatalf("não realizada: %+v, %v", d, err)
	}
	p.status = domain.DevolucaoDevolvido
	if d, err := svc.Solicitar(ctx, pedidoDev("dev2", valorDoPix)); err != nil || d.Status != domain.DevolucaoDevolvido {
		t.Fatalf("depois da não realizada, o valor todo volta a estar disponível: %+v, %v", d, err)
	}
}

func TestSolicitarDevolucaoFalhaDoPSP(t *testing.T) {
	svc, s, p, _ := novasDevolucoes(t)
	ctx := context.Background()
	p.err = errors.New("timeout")
	if _, err := svc.Solicitar(ctx, pedidoDev("dev1", 100)); !errors.Is(err, app.ErrPSP) {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.Devolucoes().Buscar(ctx, e2eLoja, "dev1"); !errors.Is(err, ports.ErrNaoEncontrado) || len(s.Eventos()) != 0 {
		t.Fatalf("gravou ou publicou depois da falha: %v, %d eventos", err, len(s.Eventos()))
	}

	p.err, p.outroValor = nil, 99
	if _, err := svc.Solicitar(ctx, pedidoDev("dev1", 100)); !errors.Is(err, app.ErrPSP) {
		t.Fatalf("PSP respondeu outro valor: %v", err)
	}
	p.outroValor, p.status = 0, "ESTRANHO"
	if _, err := svc.Solicitar(ctx, pedidoDev("dev1", 100)); !errors.Is(err, app.ErrPSP) {
		t.Fatalf("status desconhecido: %v", err)
	}
}

func TestSolicitarDevolucoesConcorrentesNaoPassamDoValor(t *testing.T) {
	svc, s, p, _ := novasDevolucoes(t)
	ctx := context.Background()
	p.espera = 20 * time.Millisecond
	var wg sync.WaitGroup
	for i := range 5 {
		wg.Go(func() { _, _ = svc.Solicitar(ctx, pedidoDev("dev"+strconv.Itoa(i), 4000)) })
	}
	wg.Wait()
	feitas, err := s.Devolucoes().DoPix(ctx, e2eLoja)
	if err != nil || len(feitas) != 2 {
		t.Fatalf("de 5 devoluções de 40,00 num Pix de 100,00, %d passaram (%v)", len(feitas), err)
	}
}

func TestConsultarDevolucao(t *testing.T) {
	svc, _, _, _ := novasDevolucoes(t)
	ctx := context.Background()
	if _, err := svc.Solicitar(ctx, pedidoDev("dev1", 100)); err != nil {
		t.Fatal(err)
	}
	if d, err := svc.Consultar(app.ComAcesso(ctx, app.Acesso{ContaID: "loja"}), e2eLoja, "dev1"); err != nil || d.ID != "dev1" {
		t.Fatalf("da loja: %+v, %v", d, err)
	}
	if _, err := svc.Consultar(app.ComAcesso(ctx, app.Acesso{ContaID: "outra"}), e2eLoja, "dev1"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("de outra conta: %v", err)
	}
	if _, err := svc.Consultar(ctx, e2eLoja, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("inexistente: %v", err)
	}
}
