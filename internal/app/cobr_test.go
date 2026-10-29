package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// pspCobr é um PSP falso com cobrança recorrente.
type pspCobr struct {
	ports.PSP
	err      error
	chamadas int
	// outroTxid faz CriarCobr devolver um txid diferente do enviado.
	outroTxid domain.Txid
	// tentativaNoBanco é o status que SolicitarRetentativa devolve; vazio não devolve tentativa.
	tentativaNoBanco domain.StatusTentativa
	// antes roda antes de o PSP responder, para simular o aviso do banco chegando no meio.
	antes func()
}

func (p *pspCobr) Nome() string { return "banco-cobr" }
func (p *pspCobr) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapRec: true, ports.CapCobr: true}
}

func (p *pspCobr) ListarPix(context.Context, domain.Conta, ports.Janela) ([]domain.PixRecebido, error) {
	return nil, nil
}

func (p *pspCobr) chamar() error {
	p.chamadas++
	if p.antes != nil {
		p.antes()
	}
	return p.err
}

func (p *pspCobr) CriarCobr(_ context.Context, _ domain.Conta, c domain.CobrancaRecorrente) (domain.CobrancaRecorrente, error) {
	if err := p.chamar(); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if p.outroTxid != "" {
		c.Txid = p.outroTxid
	}
	return c, nil
}

func (p *pspCobr) ConsultarCobr(context.Context, domain.Conta, domain.Txid) (domain.CobrancaRecorrente, error) {
	return domain.CobrancaRecorrente{}, ports.ErrNaoEncontrado
}

func (p *pspCobr) CancelarCobr(_ context.Context, _ domain.Conta, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	if err := p.chamar(); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	return domain.CobrancaRecorrente{Txid: txid, Status: domain.CobrCancelada, MotivoEncerramento: "SLCR cancelada pelo recebedor"}, nil
}

func (p *pspCobr) SolicitarRetentativa(_ context.Context, _ domain.Conta, txid domain.Txid, data domain.Data) (domain.CobrancaRecorrente, error) {
	if err := p.chamar(); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	c := domain.CobrancaRecorrente{Txid: txid, Status: domain.CobrAtiva}
	if p.tentativaNoBanco != "" {
		c.Tentativas = []domain.Tentativa{{DataLiquidacao: data, Tipo: domain.TipoNovaTentativa, Status: p.tentativaNoBanco}}
	}
	return c, nil
}

// soRec faz recorrência mas não cobrança recorrente.
type soRec struct{ ports.PSP }

func (soRec) Nome() string                     { return "so-rec" }
func (soRec) Capabilities() ports.Capabilities { return ports.Capabilities{ports.CapRec: true} }

// declaraCobr declara cobr sem implementar.
type declaraCobr struct{ ports.PSP }

func (declaraCobr) Nome() string { return "declara-cobr" }
func (declaraCobr) Capabilities() ports.Capabilities {
	return ports.Capabilities{ports.CapCobr: true}
}

// agoraCobr é quinta, 29/10/2026, 9h em Brasília: a janela de antecedência vai de 31/10 a 08/11.
var agoraCobr = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

const txidCobr = domain.Txid("cobr0000000000000000000000001")

// idRecDaConta dá a recorrência aprovada de cada conta do teste.
var idRecDaConta = map[string]string{
	"loja": "RR1234567820261029abcDEF00001", "outra": "RR1234567820261029abcDEF00002",
	"sem-cobr": "RR1234567820261029abcDEF00003", "mentirosa": "RR1234567820261029abcDEF00004",
}

func novasCobrs(t *testing.T) (app.CobrancasRecorrentes, *fake.Store, *pspCobr, *fake.Clock) {
	t.Helper()
	s, ctx := fake.NewStore(), context.Background()
	p := &pspCobr{}
	psps := map[string]string{"loja": "banco-cobr", "outra": "banco-cobr", "sem-cobr": "so-rec", "mentirosa": "declara-cobr"}
	for conta, nomePSP := range psps {
		if err := s.Contas().Criar(ctx, domain.Conta{ID: conta, Nome: conta, PSP: nomePSP, ChavePix: conta + "@example.com", Ambiente: domain.Producao, Ativa: true}); err != nil {
			t.Fatal(err)
		}
		r := pedidoRec()
		r.ID, r.ContaID, r.Status, r.DataInicial = idRecDaConta[conta], conta, domain.RecorrenciaAprovada, domain.NovaData(2026, 10, 5)
		if err := s.Recorrencias().Criar(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	clock := fake.NewClock(agoraCobr)
	return app.CobrancasRecorrentes{Store: s, PSPs: psp.NewRegistry(p, soRec{}, declaraCobr{}), Clock: clock}, s, p, clock
}

func pedidoCobr() domain.CobrancaRecorrente {
	return domain.CobrancaRecorrente{Txid: txidCobr, Valor: 9990, Vencimento: domain.NovaData(2026, 11, 5), AjusteDiaUtil: true}
}

func dadosCobr(t *testing.T, s *fake.Store, tipo domain.TipoEvento) []app.DadosCobrancaRecorrente {
	t.Helper()
	var out []app.DadosCobrancaRecorrente
	for _, raw := range eventos(t, s, tipo) {
		var d app.DadosCobrancaRecorrente
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func TestCriarCobrGravaEPublica(t *testing.T) {
	svc, s, p, _ := novasCobrs(t)
	ctx := context.Background()
	c, err := svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
	if err != nil {
		t.Fatal(err)
	}
	if c.Txid != txidCobr || c.Status != domain.CobrCriada || c.ContaID != "loja" || c.RecorrenciaID != idRecDaConta["loja"] || !c.CriadaEm.Equal(agoraCobr) || p.chamadas != 1 {
		t.Fatalf("criada = %+v, chamadas = %d", c, p.chamadas)
	}
	if lida, err := svc.Consultar(ctx, txidCobr); err != nil || lida.Valor != 9990 || !lida.AjusteDiaUtil {
		t.Fatalf("Consultar = %+v, %v", lida, err)
	}
	evs := dadosCobr(t, s, domain.EventoCobrRCriada)
	if len(evs) != 1 || evs[0].Txid != string(txidCobr) || evs[0].Status != "CRIADA" || evs[0].Valor != "99.90" || evs[0].Vencimento != "2026-11-05" {
		t.Fatalf("eventos = %+v", evs)
	}

	// Sem txid, o Lastro gera um.
	semTxid := pedidoCobr()
	semTxid.Txid = ""
	gerada, err := svc.Criar(ctx, idRecDaConta["outra"], semTxid)
	if err != nil || len(gerada.Txid) != 32 {
		t.Fatalf("txid gerado: %+v, %v", gerada, err)
	}
}

func TestCriarCobrRecusa(t *testing.T) {
	ctx := context.Background()
	casos := map[string]struct {
		prepara func(*testing.T, app.CobrancasRecorrentes, *fake.Store, *pspCobr) context.Context
		idRec   string
		muda    func(*domain.CobrancaRecorrente)
		quer    error
	}{
		"recorrência inexistente": {idRec: "RR1234567820261029abcDEF99999", quer: ports.ErrNaoEncontrado},
		"recorrência de outra conta": {
			prepara: func(*testing.T, app.CobrancasRecorrentes, *fake.Store, *pspCobr) context.Context {
				return app.ComAcesso(ctx, app.Acesso{ContaID: "outra"})
			},
			quer: ports.ErrNaoEncontrado,
		},
		"recorrência ainda não aprovada": {
			prepara: func(t *testing.T, _ app.CobrancasRecorrentes, s *fake.Store, _ *pspCobr) context.Context {
				r, _ := s.Recorrencias().Buscar(ctx, idRecDaConta["loja"])
				r.Status = domain.RecorrenciaCriada
				if err := s.Recorrencias().Atualizar(ctx, &r); err != nil {
					t.Fatal(err)
				}
				return ctx
			},
			quer: domain.ErrCobrancaRecorrenteInvalida,
		},
		"conta desativada": {
			prepara: func(_ *testing.T, _ app.CobrancasRecorrentes, s *fake.Store, _ *pspCobr) context.Context {
				c, _ := s.Contas().Buscar(ctx, "loja")
				c.Ativa = false
				_ = s.Contas().Atualizar(ctx, c)
				return ctx
			},
			quer: domain.ErrContaInativa,
		},
		"fora da antecedência":    {muda: func(c *domain.CobrancaRecorrente) { c.Vencimento = domain.NovaData(2026, 10, 30) }, quer: domain.ErrCobrancaRecorrenteInvalida},
		"valor diferente do fixo": {muda: func(c *domain.CobrancaRecorrente) { c.Valor = 100 }, quer: domain.ErrCobrancaRecorrenteInvalida},
		"txid fora do formato":    {muda: func(c *domain.CobrancaRecorrente) { c.Txid = "curto" }, quer: app.ErrRequisicaoInvalida},
		"txid de uma cob": {
			prepara: func(_ *testing.T, _ app.CobrancasRecorrentes, s *fake.Store, _ *pspCobr) context.Context {
				_ = s.Cobrancas().Criar(ctx, domain.Cobranca{Txid: txidCobr, ContaID: "loja", Status: domain.CobrancaAtiva, Valor: 100})
				return ctx
			},
			quer: ports.ErrJaExiste,
		},
		"ciclo que já tem cobrança": {
			prepara: func(t *testing.T, svc app.CobrancasRecorrentes, _ *fake.Store, p *pspCobr) context.Context {
				outra := pedidoCobr()
				outra.Txid, outra.Vencimento = "cobr0000000000000000000000002", domain.NovaData(2026, 11, 8)
				if _, err := svc.Criar(ctx, idRecDaConta["loja"], outra); err != nil {
					t.Fatal(err)
				}
				p.chamadas = 0
				return ctx
			},
			quer: domain.ErrCobrancaRecorrenteInvalida,
		},
		"PSP sem cobrança recorrente":     {idRec: idRecDaConta["sem-cobr"], quer: ports.ErrNaoSuportado},
		"PSP que declara sem implementar": {idRec: idRecDaConta["mentirosa"], quer: ports.ErrNaoSuportado},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			svc, s, p, _ := novasCobrs(t)
			ctxCaso := ctx
			if caso.prepara != nil {
				ctxCaso = caso.prepara(t, svc, s, p)
			}
			idRec := caso.idRec
			if idRec == "" {
				idRec = idRecDaConta["loja"]
			}
			pedido := pedidoCobr()
			if caso.muda != nil {
				caso.muda(&pedido)
			}
			if _, err := svc.Criar(ctxCaso, idRec, pedido); !errors.Is(err, caso.quer) {
				t.Fatalf("err = %v; quer %v", err, caso.quer)
			}
			if p.chamadas != 0 {
				t.Fatalf("chamou o PSP %d vez(es)", p.chamadas)
			}
		})
	}
}

func TestCriarCobrDepoisDeCanceladaNoMesmoCiclo(t *testing.T) {
	svc, _, _, _ := novasCobrs(t)
	ctx := context.Background()
	if _, err := svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr()); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Cancelar(ctx, txidCobr); err != nil {
		t.Fatal(err)
	}
	outra := pedidoCobr()
	outra.Txid = "cobr0000000000000000000000002"
	if _, err := svc.Criar(ctx, idRecDaConta["loja"], outra); err != nil {
		t.Fatalf("ciclo com a cobrança cancelada: %v", err)
	}
}

func TestCriarCobrPSPFalhaNadaGravado(t *testing.T) {
	ctx := context.Background()
	for nome, estraga := range map[string]func(*pspCobr){
		"falha":      func(p *pspCobr) { p.err = errors.New("banco fora do ar") },
		"outro txid": func(p *pspCobr) { p.outroTxid = "cobr0000000000000000000009999" },
	} {
		t.Run(nome, func(t *testing.T) {
			svc, s, p, _ := novasCobrs(t)
			estraga(p)
			if _, err := svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr()); !errors.Is(err, app.ErrPSP) {
				t.Fatalf("err = %v", err)
			}
			if _, err := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr); !errors.Is(err, ports.ErrNaoEncontrado) {
				t.Fatalf("gravou: %v", err)
			}
			if evs := eventos(t, s, domain.EventoCobrRCriada); len(evs) != 0 {
				t.Fatalf("publicou: %s", evs)
			}
		})
	}
}

func TestCancelarCobr(t *testing.T) {
	ctx := context.Background()
	t.Run("cancela no PSP, grava e publica", func(t *testing.T) {
		svc, s, _, _ := novasCobrs(t)
		_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
		c, err := svc.Cancelar(ctx, txidCobr)
		if err != nil || c.Status != domain.CobrCancelada || c.MotivoEncerramento == "" || c.Revisao != 1 {
			t.Fatalf("Cancelar = %+v, %v", c, err)
		}
		evs := dadosCobr(t, s, domain.EventoCobrRAtualizada)
		if len(evs) != 1 || evs[0].Status != "CANCELADA" || evs[0].MotivoEncerramento == "" {
			t.Fatalf("eventos = %+v", evs)
		}
	})
	t.Run("concluída não chega ao PSP", func(t *testing.T) {
		svc, s, p, _ := novasCobrs(t)
		_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
		c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr)
		c.Status = domain.CobrConcluida
		_ = s.CobrancasRecorrentes().Atualizar(ctx, &c)
		p.chamadas = 0
		if _, err := svc.Cancelar(ctx, txidCobr); !errors.Is(err, domain.ErrTransicaoInvalida) || p.chamadas != 0 {
			t.Fatalf("err = %v, chamadas = %d", err, p.chamadas)
		}
	})
	t.Run("de outra conta é inexistente", func(t *testing.T) {
		svc, _, _, _ := novasCobrs(t)
		_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
		if _, err := svc.Cancelar(app.ComAcesso(ctx, app.Acesso{ContaID: "outra"}), txidCobr); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("o aviso do banco chegou antes: não publica de novo", func(t *testing.T) {
		svc, s, p, _ := novasCobrs(t)
		_, _ = svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr())
		p.antes = func() {
			c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr)
			c.Status = domain.CobrCancelada
			_ = s.CobrancasRecorrentes().Atualizar(ctx, &c)
		}
		if c, err := svc.Cancelar(ctx, txidCobr); err != nil || c.Status != domain.CobrCancelada {
			t.Fatalf("Cancelar = %+v, %v", c, err)
		}
		if evs := eventos(t, s, domain.EventoCobrRAtualizada); len(evs) != 0 {
			t.Fatalf("publicou de novo: %s", evs)
		}
	})
}

// vencidaNoStore cria a cobrança de 05/11, que não liquidou, e põe o relógio em 05/11 às 15h.
func vencidaNoStore(t *testing.T, svc app.CobrancasRecorrentes, s *fake.Store, clock *fake.Clock) {
	t.Helper()
	ctx := context.Background()
	if _, err := svc.Criar(ctx, idRecDaConta["loja"], pedidoCobr()); err != nil {
		t.Fatal(err)
	}
	c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr)
	c.Status = domain.CobrAtiva
	c.Tentativas = []domain.Tentativa{{DataLiquidacao: c.Vencimento, Tipo: domain.TipoAgendamento, Status: domain.TentativaRejeitada}}
	if err := s.CobrancasRecorrentes().Atualizar(ctx, &c); err != nil {
		t.Fatal(err)
	}
	clock.Avancar(time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC).Sub(agoraCobr))
}

func TestSolicitarRetentativa(t *testing.T) {
	ctx := context.Background()
	t.Run("grava a tentativa e publica", func(t *testing.T) {
		svc, s, _, clock := novasCobrs(t)
		vencidaNoStore(t, svc, s, clock)
		c, err := svc.SolicitarRetentativa(ctx, txidCobr, domain.NovaData(2026, 11, 7))
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Tentativas) != 2 || c.Tentativas[1] != (domain.Tentativa{DataLiquidacao: domain.NovaData(2026, 11, 7), Tipo: domain.TipoNovaTentativa, Status: domain.TentativaSolicitada}) {
			t.Fatalf("tentativas = %+v", c.Tentativas)
		}
		evs := dadosCobr(t, s, domain.EventoCobrRTentativa)
		if len(evs) != 1 || evs[0].Tentativa == nil || *evs[0].Tentativa != (app.DadosTentativa{DataLiquidacao: "2026-11-07", Tipo: "NTAG", Status: "SOLICITADA"}) {
			t.Fatalf("eventos = %+v", evs)
		}
	})
	t.Run("o status da tentativa vem do banco", func(t *testing.T) {
		svc, s, p, clock := novasCobrs(t)
		vencidaNoStore(t, svc, s, clock)
		p.tentativaNoBanco = domain.TentativaAgendada
		c, err := svc.SolicitarRetentativa(ctx, txidCobr, domain.NovaData(2026, 11, 7))
		if err != nil || c.Tentativas[1].Status != domain.TentativaAgendada {
			t.Fatalf("SolicitarRetentativa = %+v, %v", c.Tentativas, err)
		}
	})
	t.Run("fora da RN-42 não chega ao PSP", func(t *testing.T) {
		svc, s, p, clock := novasCobrs(t)
		vencidaNoStore(t, svc, s, clock)
		p.chamadas = 0
		if _, err := svc.SolicitarRetentativa(ctx, txidCobr, domain.NovaData(2026, 11, 13)); !errors.Is(err, domain.ErrRetentativaRecusada) || p.chamadas != 0 {
			t.Fatalf("err = %v, chamadas = %d", err, p.chamadas)
		}
	})
	t.Run("PSP falha: nada gravado", func(t *testing.T) {
		svc, s, p, clock := novasCobrs(t)
		vencidaNoStore(t, svc, s, clock)
		p.err = errors.New("banco fora do ar")
		if _, err := svc.SolicitarRetentativa(ctx, txidCobr, domain.NovaData(2026, 11, 7)); !errors.Is(err, app.ErrPSP) {
			t.Fatalf("err = %v", err)
		}
		if c, _ := s.CobrancasRecorrentes().BuscarPorTxid(ctx, txidCobr); len(c.Tentativas) != 1 {
			t.Fatalf("gravou a tentativa: %+v", c.Tentativas)
		}
	})
}
