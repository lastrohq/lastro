package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

// pspRec é um PSP falso com Pix Automático.
type pspRec struct {
	ports.PSP
	nome       string
	caps       ports.Capabilities
	idRec      string
	idSolicRec string
	err        error
	chamadas   int
	webhookRec string
	// noBanco é o status que ConsultarRecorrencia devolve; vazio é ErrNaoEncontrado.
	noBanco       domain.StatusRecorrencia
	motivoNoBanco string
	errConsulta   error
}

func (p *pspRec) Nome() string                                                  { return p.nome }
func (p *pspRec) ConfigurarWebhook(context.Context, domain.Conta, string) error { return nil }
func (p *pspRec) Capabilities() ports.Capabilities                              { return p.caps }

func (p *pspRec) CriarRecorrencia(_ context.Context, _ domain.Conta, r domain.Recorrencia) (domain.Recorrencia, error) {
	p.chamadas++
	if p.err != nil {
		return domain.Recorrencia{}, p.err
	}
	r.ID = p.idRec
	return r, nil
}

func (p *pspRec) ConsultarRecorrencia(_ context.Context, _ domain.Conta, id string) (domain.Recorrencia, error) {
	if p.errConsulta != nil {
		return domain.Recorrencia{}, p.errConsulta
	}
	if p.noBanco == "" {
		return domain.Recorrencia{}, ports.ErrNaoEncontrado
	}
	return domain.Recorrencia{ID: id, Status: p.noBanco, MotivoRejeicao: p.motivoNoBanco}, nil
}

func (p *pspRec) ListarPix(context.Context, domain.Conta, ports.Janela) ([]domain.PixRecebido, error) {
	return nil, nil
}

func (p *pspRec) SolicitarConfirmacao(_ context.Context, _ domain.Conta, s domain.SolicitacaoRecorrencia) (domain.SolicitacaoRecorrencia, error) {
	p.chamadas++
	if p.err != nil {
		return domain.SolicitacaoRecorrencia{}, p.err
	}
	s.ID, s.Status = p.idSolicRec, domain.SolicitacaoEnviada
	return s, nil
}

func (p *pspRec) ConfigurarWebhookRec(_ context.Context, _ domain.Conta, url string) error {
	p.webhookRec = url
	return nil
}

func (p *pspRec) InterpretarWebhookRec(conta domain.Conta, corpo []byte) ([]domain.AtualizacaoRecorrencia, error) {
	return psp.WebhookRecBCB(conta, corpo)
}

// soCob é um PSP sem Pix Automático.
type soCob struct{ ports.PSP }

func (soCob) Nome() string                     { return "so-cob" }
func (soCob) Capabilities() ports.Capabilities { return ports.Capabilities{ports.CapCob: true} }

func (soCob) ListarPix(context.Context, domain.Conta, ports.Janela) ([]domain.PixRecebido, error) {
	return nil, nil
}

// semRec declara rec mas não implementa a interface opcional.
type semRec struct{ ports.PSP }

func (semRec) Nome() string                     { return "declara-sem-implementar" }
func (semRec) Capabilities() ports.Capabilities { return ports.Capabilities{ports.CapRec: true} }

func (semRec) ListarPix(context.Context, domain.Conta, ports.Janela) ([]domain.PixRecebido, error) {
	return nil, nil
}

const (
	idRecPSP      = "RR1234567820261029abcDEF00001"
	idSolicRecPSP = "SC1234567820261029abcDEF00001"
)

var agoraRec = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func novasRecorrencias(t *testing.T) (app.Recorrencias, *fake.Store, *pspRec) {
	t.Helper()
	s := fake.NewStore()
	p := &pspRec{nome: "banco-rec", caps: ports.Capabilities{ports.CapRec: true}, idRec: idRecPSP, idSolicRec: idSolicRecPSP}
	ctx := context.Background()
	for id, nomePSP := range map[string]string{"loja": "banco-rec", "sem-rec": "so-cob", "mentirosa": "declara-sem-implementar"} {
		if err := s.Contas().Criar(ctx, domain.Conta{ID: id, Nome: id, PSP: nomePSP, ChavePix: id + "@example.com", Ambiente: domain.Producao, Ativa: true}); err != nil {
			t.Fatal(err)
		}
	}
	return app.Recorrencias{Store: s, PSPs: psp.NewRegistry(p, soCob{}, semRec{}), Clock: fake.NewClock(agoraRec)}, s, p
}

func pedidoRec() domain.Recorrencia {
	return domain.Recorrencia{
		Contrato: "63100862", Objeto: "Academia", Devedor: domain.Devedor{Documento: "12345678909", Nome: "Fulano"},
		DataInicial: domain.NovaData(2026, 11, 10), Periodicidade: domain.Mensal, ValorFixo: 9990, Politica: domain.Retentativa3Em7Dias,
	}
}

func eventos(t *testing.T, s *fake.Store, tipo domain.TipoEvento) []json.RawMessage {
	t.Helper()
	evs, _ := s.Outbox().NaoDistribuidos(context.Background(), 100)
	var out []json.RawMessage
	for _, e := range evs {
		if e.Tipo == tipo {
			out = append(out, e.Dados)
		}
	}
	return out
}

func TestCriarRecorrenciaGravaComOIDDoPSPEPublicaEvento(t *testing.T) {
	svc, s, _ := novasRecorrencias(t)
	ctx := context.Background()
	criada, err := svc.Criar(ctx, "loja", pedidoRec())
	if err != nil {
		t.Fatal(err)
	}
	if criada.ID != idRecPSP || criada.Status != domain.RecorrenciaCriada || criada.ContaID != "loja" || !criada.CriadaEm.Equal(agoraRec) {
		t.Fatalf("criada = %+v", criada)
	}
	if lida, err := svc.Consultar(ctx, idRecPSP); err != nil || lida.Contrato != "63100862" {
		t.Fatalf("Consultar = %+v, %v", lida, err)
	}
	evs := eventos(t, s, domain.EventoRecorrenciaCriada)
	var d app.DadosRecorrencia
	if len(evs) != 1 || json.Unmarshal(evs[0], &d) != nil || d.IDRec != idRecPSP || d.Status != "CRIADA" || d.Periodicidade != "MENSAL" {
		t.Fatalf("eventos = %s", evs)
	}
}

func TestCriarRecorrenciaRecusa(t *testing.T) {
	ctx := context.Background()
	t.Run("inválida não chega ao PSP", func(t *testing.T) {
		svc, _, p := novasRecorrencias(t)
		r := pedidoRec()
		r.Contrato = ""
		if _, err := svc.Criar(ctx, "loja", r); !errors.Is(err, domain.ErrRecorrenciaInvalida) || p.chamadas != 0 {
			t.Fatalf("err = %v, chamadas = %d", err, p.chamadas)
		}
	})
	t.Run("conta desativada", func(t *testing.T) {
		svc, s, _ := novasRecorrencias(t)
		c, _ := s.Contas().Buscar(ctx, "loja")
		c.Ativa = false
		_ = s.Contas().Atualizar(ctx, c)
		if _, err := svc.Criar(ctx, "loja", pedidoRec()); !errors.Is(err, domain.ErrContaInativa) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("PSP sem Pix Automático", func(t *testing.T) {
		svc, _, _ := novasRecorrencias(t)
		for _, conta := range []string{"sem-rec", "mentirosa"} {
			if _, err := svc.Criar(ctx, conta, pedidoRec()); !errors.Is(err, ports.ErrNaoSuportado) {
				t.Fatalf("%s: err = %v", conta, err)
			}
		}
	})
	t.Run("PSP falha ou devolve id inválido: nada gravado", func(t *testing.T) {
		svc, s, p := novasRecorrencias(t)
		p.err = errors.New("banco fora do ar")
		if _, err := svc.Criar(ctx, "loja", pedidoRec()); !errors.Is(err, app.ErrPSP) {
			t.Fatalf("falha do PSP: %v", err)
		}
		p.err, p.idRec = nil, "id-que-nao-segue-a-regra"
		if _, err := svc.Criar(ctx, "loja", pedidoRec()); !errors.Is(err, app.ErrPSP) {
			t.Fatalf("id inválido: %v", err)
		}
		if evs := eventos(t, s, domain.EventoRecorrenciaCriada); len(evs) != 0 {
			t.Fatalf("publicou evento de recorrência não gravada: %s", evs)
		}
	})
}

func TestSolicitarConfirmacao(t *testing.T) {
	svc, s, _ := novasRecorrencias(t)
	ctx := context.Background()
	_, _ = svc.Criar(ctx, "loja", pedidoRec())
	dest := domain.ContaPagador{ISPB: "18236120", Agencia: "0001", Conta: "123456"}

	sol, err := svc.SolicitarConfirmacao(ctx, idRecPSP, dest, agoraRec.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if sol.ID != idSolicRecPSP || sol.Status != domain.SolicitacaoEnviada || sol.RecorrenciaID != idRecPSP || sol.ContaID != "loja" {
		t.Fatalf("solicitação = %+v", sol)
	}
	if gravada, err := s.Solicitacoes().Buscar(ctx, idSolicRecPSP); err != nil || gravada.Destinatario != dest {
		t.Fatalf("gravada = %+v, %v", gravada, err)
	}
	if evs := eventos(t, s, domain.EventoSolicitacaoRecorrencia); len(evs) != 1 {
		t.Fatalf("eventos = %s", evs)
	}
}

func TestSolicitarConfirmacaoRecusa(t *testing.T) {
	ctx := context.Background()
	dest := domain.ContaPagador{ISPB: "18236120", Conta: "123456"}
	t.Run("recorrência inexistente", func(t *testing.T) {
		svc, _, _ := novasRecorrencias(t)
		if _, err := svc.SolicitarConfirmacao(ctx, "RR1234567820261029abcDEF99999", dest, agoraRec.Add(time.Hour)); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("recorrência que já não está CRIADA", func(t *testing.T) {
		svc, s, _ := novasRecorrencias(t)
		_, _ = svc.Criar(ctx, "loja", pedidoRec())
		r, _ := s.Recorrencias().Buscar(ctx, idRecPSP)
		r.Status = domain.RecorrenciaAprovada
		_ = s.Recorrencias().Atualizar(ctx, &r)
		if _, err := svc.SolicitarConfirmacao(ctx, idRecPSP, dest, agoraRec.Add(time.Hour)); !errors.Is(err, domain.ErrTransicaoInvalida) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("expiração fora dos 30 dias", func(t *testing.T) {
		svc, _, p := novasRecorrencias(t)
		_, _ = svc.Criar(ctx, "loja", pedidoRec())
		antes := p.chamadas
		if _, err := svc.SolicitarConfirmacao(ctx, idRecPSP, dest, agoraRec.Add(31*24*time.Hour)); !errors.Is(err, domain.ErrRecorrenciaInvalida) || p.chamadas != antes {
			t.Fatalf("err = %v, chamadas ao PSP = %d", err, p.chamadas-antes)
		}
	})
	t.Run("PSP devolve idSolicRec inválido", func(t *testing.T) {
		svc, s, p := novasRecorrencias(t)
		_, _ = svc.Criar(ctx, "loja", pedidoRec())
		p.idSolicRec = "SC-curto"
		if _, err := svc.SolicitarConfirmacao(ctx, idRecPSP, dest, agoraRec.Add(time.Hour)); !errors.Is(err, app.ErrPSP) {
			t.Fatalf("err = %v", err)
		}
		if evs := eventos(t, s, domain.EventoSolicitacaoRecorrencia); len(evs) != 0 {
			t.Fatalf("publicou evento: %s", evs)
		}
	})
}

func TestRecorrenciaDeOutraContaNaoAparece(t *testing.T) {
	svc, s, _ := novasRecorrencias(t)
	ctx := context.Background()
	_, _ = svc.Criar(ctx, "loja", pedidoRec())
	_ = s.Contas().Criar(ctx, domain.Conta{ID: "outra", Nome: "Outra", PSP: "banco-rec", ChavePix: "outra@example.com", Ambiente: domain.Producao, Ativa: true})
	daOutra := app.ComAcesso(ctx, app.Acesso{ContaID: "outra"})
	if _, err := svc.Consultar(daOutra, idRecPSP); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("consultar de outra conta: %v", err)
	}
	if _, err := svc.SolicitarConfirmacao(daOutra, idRecPSP, domain.ContaPagador{ISPB: "18236120", Conta: "1"}, agoraRec.Add(time.Hour)); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("solicitar de outra conta: %v", err)
	}
	if _, err := svc.Criar(daOutra, "loja", pedidoRec()); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("criar na conta dos outros: %v", err)
	}
	if _, err := svc.Consultar(app.ComAcesso(ctx, app.Acesso{ContaID: "loja"}), idRecPSP); err != nil {
		t.Fatalf("a própria conta não enxerga: %v", err)
	}
}

// pspDaLoja devolve o PSP falso da conta loja, para mudar o que ele responde.
func pspDaLoja(t *testing.T, svc app.Recorrencias) *pspRec {
	t.Helper()
	p, err := svc.PSPs.PSP("banco-rec")
	if err != nil {
		t.Fatal(err)
	}
	return p.(*pspRec)
}
