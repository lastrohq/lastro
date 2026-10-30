package efi_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/efi"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
	"github.com/vintemhq/vintem/internal/ports"
	"github.com/vintemhq/vintem/internal/ports/fake"
	"github.com/vintemhq/vintem/test/conformance"
	"github.com/vintemhq/vintem/test/conformance/fita"
)

// A suíte de conformidade roda contra o Efí falso, que segue os exemplos da
// documentação.
func TestConformidade(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		c := novo(t)
		relogio := fake.NewClock(agora)
		return conformance.Alvo{
			PSP:   c.psp,
			Conta: c.conta,
			Agora: relogio.Now,
			Pagar: func(_ *testing.T, txid domain.Txid) {
				c.efi.Pagar(string(txid), relogio.Now())
				relogio.Avancar(time.Second)
			},
		}
	})
}

// dirFitas guarda as fitas gravadas contra a homologação do Efí.
const dirFitas = "../../../../test/fixtures/efi"

// TestConformidadeGravada roda a suíte contra as fitas da homologação do Efí
// de verdade (RN-51). Para gravar de novo, com uma conta Efí Pro:
//
//	VINTEM_GRAVAR_EFI=1 EFI_CLIENT_ID=... EFI_CLIENT_SECRET=... \
//	EFI_CERTIFICADO=homologacao.p12 EFI_SENHA_CERTIFICADO= EFI_CHAVE_PIX=... \
//	EFI_URL_WEBHOOK=https://... \
//	go test -run TestConformidadeGravada ./internal/adapters/psp/efi/
//
// A homologação paga sozinha cobranças de até R$ 10,00: Pagar espera por isso
// consultando a cobrança, e a fita guarda quanto tempo levou.
// EFI_URL_WEBHOOK é opcional: o Efí chama a URL e só aceita se ela responder
// 2xx; sem ela, o cenário de configurar o webhook é pulado.
func TestConformidadeGravada(t *testing.T) {
	gravar := os.Getenv("VINTEM_GRAVAR_EFI") != ""
	if !gravar {
		if fitas, _ := filepath.Glob(filepath.Join(dirFitas, "*.json")); len(fitas) == 0 {
			t.Skip("nenhuma fita da homologação do Efí gravada ainda (#27)")
		}
	}
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		f := fita.Abrir(t, fita.Arquivo(dirFitas, t.Name()), gravar)
		var cred domain.Credenciais
		if gravar {
			p12, err := os.ReadFile(os.Getenv("EFI_CERTIFICADO")) //nolint:gosec // G703: quem grava escolhe o próprio certificado
			if err != nil {
				t.Fatal(err)
			}
			cred = domain.Credenciais{
				ClientID: os.Getenv("EFI_CLIENT_ID"), ClientSecret: os.Getenv("EFI_CLIENT_SECRET"),
				Certificado: p12, SenhaCertificado: os.Getenv("EFI_SENHA_CERTIFICADO"),
			}
			f.Dados = map[string]string{"chave": os.Getenv("EFI_CHAVE_PIX"), "webhook": os.Getenv("EFI_URL_WEBHOOK")}
		} else {
			// Na reprodução nada sai para a rede: o certificado só precisa abrir.
			par := certificadotest.Gerar(t, "loja", f.Agora.Add(365*24*time.Hour))
			cred = domain.Credenciais{ClientID: "gravado", ClientSecret: "gravado", Certificado: par.P12, SenhaCertificado: certificadotest.Senha}
		}
		relogio := fake.NewClock(f.Agora)
		conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "efi", ChavePix: f.Dados["chave"], Ambiente: domain.Homologacao, Ativa: true}
		p := efi.New(&credenciais{porID: map[string]domain.Credenciais{"loja": cred}}, relogio, efi.ComTransporte(f.Envolver))
		return conformance.Alvo{
			PSP: p, Conta: conta, Agora: relogio.Now, Rodada: f.Rodada, Real: true, URLWebhook: f.Dados["webhook"],
			Pagar: func(t *testing.T, txid domain.Txid) { esperarPagamento(t, f, p, conta, relogio, txid, gravar) },
		}
	})
}

// esperarPagamento consulta a cobrança enquanto ela estiver ATIVA. Gravando,
// espera de verdade e anota na fita quantos segundos a homologação levou; na
// reprodução, as consultas vêm da fita sem espera. Nos dois casos o relógio
// avança o tempo anotado, para as janelas de ListarPix saírem iguais.
func esperarPagamento(t *testing.T, f *fita.Fita, p *efi.PSP, conta domain.Conta, relogio *fake.Clock, txid domain.Txid, gravar bool) {
	t.Helper()
	ctx := context.Background()
	inicio := time.Now()
	for tentativa := 0; ; tentativa++ {
		c, err := p.ConsultarCob(ctx, conta, txid)
		status := c.Status
		if errors.Is(err, ports.ErrNaoEncontrado) {
			v, errV := p.ConsultarCobv(ctx, conta, txid)
			status, err = v.Status, errV
		}
		if err != nil {
			t.Fatalf("consultando %s: %v", txid, err)
		}
		if status != domain.CobrancaAtiva {
			break
		}
		switch {
		case !gravar && tentativa > 100:
			t.Fatalf("a fita não termina com %s paga", txid)
		case gravar && time.Since(inicio) > 2*time.Minute:
			t.Fatalf("a homologação não pagou %s em 2 min", txid)
		case gravar:
			time.Sleep(3 * time.Second)
		}
	}
	chave := "pago:" + string(txid)
	if gravar {
		f.Dados[chave] = strconv.Itoa(int(time.Since(inicio).Seconds()) + 2)
	}
	segundos, _ := strconv.Atoi(f.Dados[chave])
	relogio.Avancar(time.Duration(segundos) * time.Second)
}

// TestFitaGravaEReproduz grava a suíte contra o Efí falso e a reproduz sem
// servidor: garante que a fita funciona antes de existir gravação de verdade.
func TestFitaGravaEReproduz(t *testing.T) {
	dir := t.TempDir()
	// O nome da fita ignora a fase, para "gravar" e "reproduzir" acharem o mesmo arquivo.
	arquivo := func(t *testing.T) string { return fita.Arquivo(dir, strings.SplitN(t.Name(), "/", 3)[2]) }
	t.Run("gravar", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			c := novo(t)
			f := fita.Abrir(t, arquivo(t), true)
			f.Dados = map[string]string{"chave": c.conta.ChavePix}
			relogio := fake.NewClock(f.Agora)
			p := efi.New(c.cred, relogio, efi.ComURLs("https://producao.invalid", c.efi.URL), efi.ComRaizes(c.efi.Raizes), efi.ComTransporte(f.Envolver))
			return conformance.Alvo{PSP: p, Conta: c.conta, Agora: relogio.Now, Rodada: f.Rodada}
		})
	})
	if fitas, _ := filepath.Glob(filepath.Join(dir, "*.json")); len(fitas) < 5 {
		t.Fatalf("só %d fitas gravadas", len(fitas))
	}
	t.Run("reproduzir", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Alvo {
			f := fita.Abrir(t, arquivo(t), false)
			par := certificadotest.Gerar(t, "loja", f.Agora.Add(365*24*time.Hour))
			cred := domain.Credenciais{ClientID: "gravado", ClientSecret: "gravado", Certificado: par.P12, SenhaCertificado: certificadotest.Senha}
			relogio := fake.NewClock(f.Agora)
			conta := domain.Conta{ID: "loja", Nome: "Loja", PSP: "efi", ChavePix: f.Dados["chave"], Ambiente: domain.Homologacao, Ativa: true}
			p := efi.New(&credenciais{porID: map[string]domain.Credenciais{"loja": cred}}, relogio,
				efi.ComURLs("https://producao.invalid", "https://homologacao.invalid"), efi.ComTransporte(f.Envolver))
			return conformance.Alvo{PSP: p, Conta: conta, Agora: relogio.Now, Rodada: f.Rodada}
		})
	})
}
