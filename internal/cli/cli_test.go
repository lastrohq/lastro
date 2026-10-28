package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/platform/vault"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

type pspFalso struct{ ports.PSP }

func (pspFalso) Nome() string { return "sandbox" }

func ambiente(t *testing.T, env map[string]string) (Ambiente, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	return Ambiente{Saida: &out, Getenv: func(k string) string { return env[k] }, PSPs: psp.NewRegistry(pspFalso{}), Clock: fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)), Versao: "v-test"}, &out
}

func envComBanco(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{"LASTRO_DB": filepath.Join(t.TempDir(), "lastro.db"), "LASTRO_MASTER_KEY": vault.NovaChave()}
}

func TestVersionEChave(t *testing.T) {
	amb, out := ambiente(t, nil)
	if err := Run(context.Background(), []string{"version"}, amb); err != nil || strings.TrimSpace(out.String()) != "v-test" {
		t.Fatalf("version: %q, %v", out, err)
	}
	out.Reset()
	if err := Run(context.Background(), []string{"chave", "gerar"}, amb); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.New(strings.TrimSpace(out.String())); err != nil {
		t.Fatalf("chave gerada é inválida: %v", err)
	}
}

func TestContaSemMasterKeyFalhaComOrientacao(t *testing.T) {
	amb, _ := ambiente(t, map[string]string{"LASTRO_DB": filepath.Join(t.TempDir(), "x.db")})
	err := Run(context.Background(), []string{"conta", "listar"}, amb)
	if err == nil || !strings.Contains(err.Error(), "lastro chave gerar") {
		t.Fatalf("err = %v", err)
	}
}

func TestContaCriarListarDesativar(t *testing.T) {
	env := envComBanco(t)
	amb, out := ambiente(t, env)
	ctx := context.Background()

	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("segredo-do-arquivo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Run(ctx, []string{"conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox",
		"--chave", "x@example.com", "--client-id", "abc", "--client-secret-arquivo", secret}, amb)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "segredo-do-arquivo") {
		t.Fatal("a saída mostrou o segredo")
	}

	out.Reset()
	if err := Run(ctx, []string{"conta", "listar"}, amb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "loja") || !strings.Contains(out.String(), "true") || strings.Contains(out.String(), "segredo") {
		t.Fatalf("listar: %s", out)
	}

	out.Reset()
	if err := Run(ctx, []string{"conta", "desativar", "loja"}, amb); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	_ = Run(ctx, []string{"conta", "listar"}, amb)
	if !strings.Contains(out.String(), "false") {
		t.Fatalf("depois de desativar: %s", out)
	}

	// O segredo foi gravado sem a quebra de linha do arquivo, e só sai decifrado.
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	cred, err := contas.Credenciais(ctx, "loja")
	if err != nil || cred.ClientSecret != "segredo-do-arquivo" || cred.ClientID != "abc" {
		t.Fatalf("credenciais: id=%q, %v", cred.ClientID, err)
	}
}

func TestContaCriarComPSPDesconhecido(t *testing.T) {
	amb, _ := ambiente(t, envComBanco(t))
	err := Run(context.Background(), []string{"conta", "criar", "--id", "x", "--nome", "X", "--psp", "itau", "--chave", "k"}, amb)
	if err == nil || !strings.Contains(err.Error(), "disponíveis") {
		t.Fatalf("err = %v", err)
	}
}

func TestComandoDesconhecido(t *testing.T) {
	amb, _ := ambiente(t, nil)
	if err := Run(context.Background(), []string{"voar"}, amb); err == nil || !strings.Contains(err.Error(), "uso:") {
		t.Fatalf("err = %v", err)
	}
}

func TestAPIKeyCriarListarRevogar(t *testing.T) {
	amb, out := ambiente(t, envComBanco(t))
	ctx := context.Background()
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "loja", "--escopo", "cob:read", "--escopo", "cob:write"}, amb); err != nil {
		t.Fatal(err)
	}
	var token, id string
	for _, campo := range strings.Fields(out.String()) {
		if strings.HasPrefix(campo, "lst_") {
			token = campo
		}
	}
	id = strings.Fields(out.String())[2]
	if token == "" {
		t.Fatalf("token não apareceu: %s", out)
	}
	out.Reset()
	_ = Run(ctx, []string{"apikey", "listar"}, amb)
	if !strings.Contains(out.String(), id) || strings.Contains(out.String(), token) {
		t.Fatalf("listar: %s", out)
	}
	if err := Run(ctx, []string{"apikey", "revogar", id}, amb); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, []string{"apikey", "criar", "--nome", "x", "--escopo", "tudo"}, amb); err == nil {
		t.Fatal("aceitou escopo inventado")
	}
}
