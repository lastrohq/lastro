package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/certificado/certificadotest"
)

// arquivo grava conteudo num arquivo temporário e devolve o caminho.
func arquivo(t *testing.T, nome string, conteudo []byte) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), nome)
	if err := os.WriteFile(caminho, conteudo, 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

// O relógio do ambiente de teste marca 2026-10-29.
var validadeTeste = time.Date(2027, 6, 30, 23, 59, 59, 0, time.UTC)

func criarConta(ctx context.Context, amb Ambiente, id, cert, senha string) error {
	args := []string{"conta", "criar", "--id", id, "--nome", "Loja", "--psp", "sandbox", "--chave", id + "@example.com", "--certificado", cert}
	if senha != "" {
		args = append(args, "--senha-certificado-arquivo", senha)
	}
	return Run(ctx, args, amb)
}

func TestContaCriarComCertificado(t *testing.T) {
	par := certificadotest.Gerar(t, "loja.example.com", validadeTeste)
	for nome, c := range map[string]struct {
		cert  []byte
		senha string
	}{
		"p12":        {par.P12, certificadotest.Senha},
		"p12 legado": {par.P12Legado, certificadotest.Senha},
		"pem":        {par.PEM, ""},
	} {
		t.Run(nome, func(t *testing.T) {
			amb, out := ambiente(t, envComBanco(t))
			ctx := context.Background()
			senha := ""
			if c.senha != "" {
				senha = arquivo(t, "senha", []byte(c.senha+"\n"))
			}
			if err := criarConta(ctx, amb, "loja", arquivo(t, "cert", c.cert), senha); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "válido até 2027-06-30") || !strings.Contains(out.String(), "CN=loja.example.com") {
				t.Fatalf("criar: %s", out)
			}
			out.Reset()
			if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "CERTIFICADO VALE ATÉ") || !strings.Contains(out.String(), "2027-06-30") {
				t.Fatalf("listar: %s", out)
			}
		})
	}
}

func TestContaCriarRecusaCertificado(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", validadeTeste)
	vencido := certificadotest.Gerar(t, "velho", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	outro := certificadotest.Gerar(t, "outro", validadeTeste)
	for nome, c := range map[string]struct {
		cert   []byte
		senha  string
		quer   error
		motivo string
	}{
		"senha errada":      {par.P12, "senha-incorreta-xyz", domain.ErrCertificadoInvalido, "senha do .p12 errada"},
		"sem senha":         {par.P12, "", domain.ErrCertificadoInvalido, "senha do .p12 errada"},
		"chave de outro":    {append(append([]byte{}, par.CertificadoPEM...), outro.ChavePEM...), "", domain.ErrCertificadoInvalido, "não corresponde"},
		"vencido":           {vencido.P12, certificadotest.Senha, domain.ErrCertificadoVencido, "2026-10-01"},
		"não é certificado": {[]byte("isto não é um certificado"), "", domain.ErrCertificadoInvalido, "não é um .p12 nem um PEM"},
	} {
		t.Run(nome, func(t *testing.T) {
			amb, _ := ambiente(t, envComBanco(t))
			ctx := context.Background()
			senha := ""
			if c.senha != "" {
				senha = arquivo(t, "senha", []byte(c.senha))
			}
			err := criarConta(ctx, amb, "loja", arquivo(t, "cert", c.cert), senha)
			if !errors.Is(err, c.quer) || !strings.Contains(err.Error(), c.motivo) {
				t.Fatalf("err = %v; quer %v com %q", err, c.quer, c.motivo)
			}
			if strings.Contains(err.Error(), c.senha) && c.senha != "" {
				t.Fatalf("a mensagem mostrou a senha: %v", err)
			}
			contas, fechar, _ := abrir(ctx, amb)
			defer fechar()
			if lista, _ := contas.Listar(ctx); len(lista) != 0 {
				t.Fatal("gravou a conta com certificado recusado")
			}
		})
	}
}

func TestServeSemMasterKeyNaoSobe(t *testing.T) {
	amb, _ := ambiente(t, map[string]string{"VINTEM_DB": filepath.Join(t.TempDir(), "x.db"), "VINTEM_ADDR": "127.0.0.1:0"})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	err := Run(ctx, []string{"serve"}, amb)
	if err == nil || !strings.Contains(err.Error(), "VINTEM_MASTER_KEY") {
		t.Fatalf("serve sem master key: err = %v", err)
	}
}

// TestNenhumSegredoNoLogNemNaSaida cadastra uma conta com todos os segredos,
// erra a senha de propósito, sobe e derruba o serve, e procura cada segredo em
// tudo o que o Vintém escreveu (RN-61).
func TestNenhumSegredoNoLogNemNaSaida(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", validadeTeste)
	env := envComBanco(t)
	env["VINTEM_ADDR"] = "127.0.0.1:0"
	env["VINTEM_CONCILIACAO_INTERVALO"] = "0"
	env["VINTEM_WEBHOOK_INTERVALO"] = "0"
	amb, out := ambiente(t, env)

	var log bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	const clientSecret = "client-secret-que-nao-pode-vazar"
	secret := arquivo(t, "secret", []byte(clientSecret))
	senha := arquivo(t, "senha", []byte(certificadotest.Senha))
	cert := arquivo(t, "cert.p12", par.P12)
	ctx := context.Background()

	// Primeiro com a senha errada, para passar pelo caminho de erro.
	errada := arquivo(t, "errada", []byte("senha-errada-que-nao-pode-vazar"))
	errCriar := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "x@example.com",
		"--client-id", "abc", "--client-secret-arquivo", secret, "--certificado", cert, "--senha-certificado-arquivo", errada}, amb)
	if errCriar == nil {
		t.Fatal("aceitou a senha errada")
	}
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "x@example.com",
		"--client-id", "abc", "--client-secret-arquivo", secret, "--certificado", cert, "--senha-certificado-arquivo", senha}, amb); err != nil {
		t.Fatal(err)
	}
	_ = Run(ctx, []string{"conta", "listar"}, amb)

	servindo, parar := context.WithCancel(ctx)
	feito := make(chan error, 1)
	go func() { feito <- Run(servindo, []string{"serve"}, amb) }()
	time.Sleep(200 * time.Millisecond)
	parar()
	if err := <-feito; err != nil {
		t.Fatalf("serve: %v", err)
	}
	if !strings.Contains(log.String(), "vintem no ar") {
		t.Fatalf("o serve não chegou a subir; log: %s", log.String())
	}

	tudo := out.String() + log.String() + errCriar.Error()
	segredos := map[string]string{
		"client secret":            clientSecret,
		"senha do certificado":     certificadotest.Senha,
		"senha errada":             "senha-errada-que-nao-pode-vazar",
		"chave privada (PEM)":      strings.Split(string(par.ChavePEM), "\n")[1],
		"certificado .p12 (bruto)": string(par.P12[20:40]),
	}
	for nome, s := range segredos {
		if strings.Contains(tudo, s) {
			t.Errorf("%s apareceu no log ou na saída", nome)
		}
	}
}

// TestServeAvisaCertificadoPertoDeVencer cadastra uma conta com um certificado
// real que vence em 20 dias, um destino de webhook, e sobe o serve: o evento
// certificado.expirando tem que chegar ao destino, uma vez só.
func TestServeAvisaCertificadoPertoDeVencer(t *testing.T) {
	recebidos := make(chan string, 10)
	destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		recebidos <- string(corpo)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer destino.Close()

	env := envComBanco(t)
	env["VINTEM_ADDR"] = "127.0.0.1:0"
	env["VINTEM_CONCILIACAO_INTERVALO"] = "0"
	env["VINTEM_WEBHOOK_INTERVALO"] = "50ms"
	amb, _ := ambiente(t, env) // relógio em 2026-10-29 12:00 UTC
	ctx := context.Background()

	par := certificadotest.Gerar(t, "loja", time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
	if err := criarConta(ctx, amb, "loja", arquivo(t, "cert.p12", par.P12), arquivo(t, "senha", []byte(certificadotest.Senha))); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"destino", "criar", "--url", destino.URL}, amb); err != nil {
		t.Fatal(err)
	}

	servindo, parar := context.WithCancel(ctx)
	feito := make(chan error, 1)
	go func() { feito <- Run(servindo, []string{"serve"}, amb) }()
	defer func() {
		parar()
		if err := <-feito; err != nil {
			t.Errorf("serve: %v", err)
		}
	}()

	select {
	case corpo := <-recebidos:
		var ev struct {
			Type string               `json:"type"`
			Data app.DadosCertificado `json:"data"`
		}
		if err := json.Unmarshal([]byte(corpo), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Type != "certificado.expirando" || ev.Data.ContaID != "loja" || ev.Data.DiasRestantes != 20 || ev.Data.Sujeito != "CN=loja,O=Vintém Teste" {
			t.Fatalf("evento = %s", corpo)
		}
		if strings.Contains(corpo, string(par.CertificadoPEM[30:60])) {
			t.Fatal("o evento levou o certificado")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o webhook certificado.expirando não chegou em 5 s")
	}
	select {
	case corpo := <-recebidos:
		t.Fatalf("chegou um segundo webhook: %s", corpo)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestCertificadoDeHomologacaoRecusadoEmProducaoPelaCLI(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", validadeTeste)
	amb, _ := ambiente(t, envComBanco(t))
	ctx := context.Background()
	cert, senha := arquivo(t, "cert.p12", par.P12), arquivo(t, "senha", []byte(certificadotest.Senha))
	criar := func(id, ambiente string) error {
		return Run(ctx, []string{"conta", "criar", "--id", id, "--nome", "Loja", "--psp", "sandbox", "--chave", id + "@example.com",
			"--ambiente", ambiente, "--certificado", cert, "--senha-certificado-arquivo", senha}, amb)
	}
	if err := criar("loja-homolog", "homologacao"); err != nil {
		t.Fatal(err)
	}
	err := criar("loja-prod", "producao")
	if !errors.Is(err, domain.ErrCertificadoDeHomologacao) || !strings.Contains(err.Error(), "loja-homolog") {
		t.Fatalf("err = %v", err)
	}
	// Com o certificado de produção, a mesma conta entra.
	prod := certificadotest.Gerar(t, "loja", validadeTeste)
	if err := Run(ctx, []string{"conta", "criar", "--id", "loja-prod", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja-prod@example.com",
		"--ambiente", "producao", "--certificado", arquivo(t, "prod.p12", prod.P12), "--senha-certificado-arquivo", senha}, amb); err != nil {
		t.Fatalf("certificado de produção recusado: %v", err)
	}
}
