package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/vault"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

type pspFalso struct{ ports.PSP }

func (pspFalso) Nome() string { return "sandbox" }

func novoContas(t *testing.T) (app.Contas, *fake.Store) {
	t.Helper()
	v, err := vault.New(vault.NovaChave())
	if err != nil {
		t.Fatal(err)
	}
	s := fake.NewStore()
	return app.Contas{
		Store: s, Vault: v, PSPs: psp.NewRegistry(pspFalso{}),
		Certificados: fake.Certificados{Certificado: certValido}, Clock: fake.NewClock(agora),
	}, s
}

var (
	agora      = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	certValido = domain.Certificado{Sujeito: "CN=loja", ImpressaoDigital: "ab12", Validade: agora.Add(90 * 24 * time.Hour)}
)

var cred = domain.Credenciais{ClientID: "id-123", ClientSecret: "segredo-xyz", Certificado: []byte("PEM...")}

func conta() domain.Conta {
	return domain.Conta{ID: "loja", Nome: "Loja", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: domain.Homologacao}
}

func TestCriarContaGravaAtivaComCredenciaisCifradas(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	if err := svc.Criar(ctx, conta(), cred); err != nil {
		t.Fatal(err)
	}
	c, err := s.Contas().Buscar(ctx, "loja")
	if err != nil || !c.Ativa {
		t.Fatalf("conta = %+v, %v", c, err)
	}
	blob, _ := s.Contas().BuscarCredenciais(ctx, "loja")
	if bytes.Contains(blob, []byte("segredo-xyz")) || bytes.Contains(blob, []byte("id-123")) {
		t.Fatal("credencial gravada em claro")
	}
	got, err := svc.Credenciais(ctx, "loja")
	if err != nil || got.ClientSecret != cred.ClientSecret || string(got.Certificado) != "PEM..." {
		t.Fatalf("Credenciais = %v, %v", got.ClientID, err)
	}
}

func TestCriarContaRecusa(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()

	invalida := conta()
	invalida.ID = "Com Espaço"
	if err := svc.Criar(ctx, invalida, cred); !errors.Is(err, domain.ErrContaInvalida) {
		t.Fatalf("conta inválida: err = %v", err)
	}
	outroPSP := conta()
	outroPSP.PSP = "banco-inventado"
	if err := svc.Criar(ctx, outroPSP, cred); !errors.Is(err, ports.ErrPSPDesconhecido) {
		t.Fatalf("psp desconhecido: err = %v", err)
	}
	if lista, _ := s.Contas().Listar(ctx); len(lista) != 0 {
		t.Fatalf("gravou conta recusada: %+v", lista)
	}
	_ = svc.Criar(ctx, conta(), cred)
	if err := svc.Criar(ctx, conta(), cred); !errors.Is(err, ports.ErrJaExiste) {
		t.Fatalf("duplicada: err = %v", err)
	}
}

func TestDesativarContaImpedeCobranca(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	_ = svc.Criar(ctx, conta(), cred)
	if err := svc.Desativar(ctx, "loja"); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Contas().Buscar(ctx, "loja")
	if err := c.PodeCobrar(); !errors.Is(err, domain.ErrContaInativa) {
		t.Fatalf("conta desativada ainda cobra: %v", err)
	}
	if err := svc.Desativar(ctx, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("inexistente: err = %v", err)
	}
}

func TestCredenciaisComOutraMasterKeyFalham(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	_ = svc.Criar(ctx, conta(), cred)
	outra, _ := vault.New(vault.NovaChave())
	svc2 := app.Contas{Store: s, Vault: outra, PSPs: svc.PSPs}
	if _, err := svc2.Credenciais(ctx, "loja"); !errors.Is(err, vault.ErrDecifrar) {
		t.Fatalf("err = %v", err)
	}
}

func TestCriarContaGuardaOsDadosPublicosDoCertificado(t *testing.T) {
	svc, s := novoContas(t)
	ctx := context.Background()
	if err := svc.Criar(ctx, conta(), cred); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Contas().Buscar(ctx, "loja")
	if c.Certificado == nil || !reflect.DeepEqual(*c.Certificado, certValido) {
		t.Fatalf("Certificado = %+v; quer %+v", c.Certificado, certValido)
	}
}

func TestCriarContaSemCertificadoNaoPrecisaDoLeitor(t *testing.T) {
	svc, s := novoContas(t)
	svc.Certificados, svc.Clock = nil, nil // o sandbox não tem certificado
	ctx := context.Background()
	if err := svc.Criar(ctx, conta(), domain.Credenciais{ClientID: "x"}); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Contas().Buscar(ctx, "loja"); c.Certificado != nil {
		t.Fatalf("conta sem certificado ganhou %+v", c.Certificado)
	}
}

func TestCriarContaRecusaCertificado(t *testing.T) {
	for nome, c := range map[string]struct {
		leitor fake.Certificados
		quer   error
		trecho string
	}{
		"ilegível":    {fake.Certificados{Err: fmt.Errorf("%w: senha do .p12 errada", domain.ErrCertificadoInvalido)}, domain.ErrCertificadoInvalido, "senha do .p12 errada"},
		"vencido":     {fake.Certificados{Certificado: domain.Certificado{Sujeito: "CN=loja", Validade: agora.Add(-time.Hour)}}, domain.ErrCertificadoVencido, "2026-10-29"},
		"vence agora": {fake.Certificados{Certificado: domain.Certificado{Sujeito: "CN=loja", Validade: agora}}, domain.ErrCertificadoVencido, "CN=loja"},
	} {
		t.Run(nome, func(t *testing.T) {
			svc, s := novoContas(t)
			svc.Certificados = c.leitor
			ctx := context.Background()
			err := svc.Criar(ctx, conta(), cred)
			if !errors.Is(err, c.quer) || !errors.Is(err, domain.ErrContaInvalida) {
				t.Fatalf("err = %v; quer %v (e ErrContaInvalida)", err, c.quer)
			}
			if !strings.Contains(err.Error(), c.trecho) {
				t.Fatalf("err %q não diz %q", err, c.trecho)
			}
			if lista, _ := s.Contas().Listar(ctx); len(lista) != 0 {
				t.Fatal("gravou a conta com certificado recusado")
			}
		})
	}
}

func TestCriarContaComCertificadoSemLeitorFalhaFechado(t *testing.T) {
	svc, _ := novoContas(t)
	svc.Certificados = nil
	if err := svc.Criar(context.Background(), conta(), cred); err == nil {
		t.Fatal("aceitou certificado sem conseguir conferir")
	}
}

func contaEm(id string, ambiente domain.Ambiente) domain.Conta {
	return domain.Conta{ID: id, Nome: "Loja", PSP: "sandbox", ChavePix: id + "@example.com", Ambiente: ambiente}
}

func TestCertificadoDeHomologacaoNaoEntraEmProducao(t *testing.T) {
	svc, s := novoContas(t) // o leitor falso devolve sempre o mesmo certificado
	ctx := context.Background()
	if err := svc.Criar(ctx, contaEm("loja-homolog", domain.Homologacao), cred); err != nil {
		t.Fatal(err)
	}
	err := svc.Criar(ctx, contaEm("loja-prod", domain.Producao), cred)
	if !errors.Is(err, domain.ErrCertificadoDeHomologacao) || !errors.Is(err, domain.ErrContaInvalida) {
		t.Fatalf("err = %v; quer ErrCertificadoDeHomologacao", err)
	}
	if !strings.Contains(err.Error(), "loja-homolog") {
		t.Fatalf("a mensagem não diz qual conta de homologação tem o certificado: %v", err)
	}
	if _, err := s.Contas().Buscar(ctx, "loja-prod"); !errors.Is(err, ports.ErrNaoEncontrado) {
		t.Fatalf("a conta recusada foi gravada: %v", err)
	}
}

func TestCertificadoEAmbientePermitidos(t *testing.T) {
	outroCert := certValido
	outroCert.ImpressaoDigital = "cd34"
	for nome, c := range map[string]struct {
		primeira, segunda domain.Ambiente
		certSegunda       domain.Certificado
	}{
		"produção e depois homologação":  {domain.Producao, domain.Homologacao, certValido},
		"duas de homologação":            {domain.Homologacao, domain.Homologacao, certValido},
		"duas de produção":               {domain.Producao, domain.Producao, certValido},
		"produção com outro certificado": {domain.Homologacao, domain.Producao, outroCert},
	} {
		t.Run(nome, func(t *testing.T) {
			svc, _ := novoContas(t)
			ctx := context.Background()
			if err := svc.Criar(ctx, contaEm("a", c.primeira), cred); err != nil {
				t.Fatal(err)
			}
			svc.Certificados = fake.Certificados{Certificado: c.certSegunda}
			if err := svc.Criar(ctx, contaEm("b", c.segunda), cred); err != nil {
				t.Fatalf("recusou: %v", err)
			}
		})
	}
}

func TestContaDeHomologacaoDesativadaAindaMarcaOCertificado(t *testing.T) {
	svc, _ := novoContas(t)
	ctx := context.Background()
	_ = svc.Criar(ctx, contaEm("velha", domain.Homologacao), cred)
	if err := svc.Desativar(ctx, "velha"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Criar(ctx, contaEm("prod", domain.Producao), cred); !errors.Is(err, domain.ErrCertificadoDeHomologacao) {
		t.Fatalf("err = %v; o certificado continua sendo de homologação mesmo com a conta desativada", err)
	}
}

func TestContaDeProducaoSemCertificadoNaoEConferida(t *testing.T) {
	svc, _ := novoContas(t)
	ctx := context.Background()
	_ = svc.Criar(ctx, contaEm("homolog", domain.Homologacao), cred)
	if err := svc.Criar(ctx, contaEm("prod", domain.Producao), domain.Credenciais{ClientID: "x"}); err != nil {
		t.Fatalf("conta sem certificado recusada: %v", err)
	}
}
