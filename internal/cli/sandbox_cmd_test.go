package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/vault"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// saidaSegura deixa o teste ler a saída enquanto o servidor ainda roda.
type saidaSegura struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *saidaSegura) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *saidaSegura) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

var reToken = regexp.MustCompile(`export LASTRO_TOKEN=(\S+)`)

// subirSandbox roda `lastro sandbox` até o servidor responder e devolve o token
// impresso e a função que derruba o processo.
func subirSandbox(t *testing.T, db string, relogio ports.Clock) (url, token string, parar func()) {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	env := map[string]string{"LASTRO_DB": db, "LASTRO_ADDR": addr, "LASTRO_WEBHOOK_INTERVALO": "0", "LASTRO_CERTIFICADO_INTERVALO": "0"}
	saida := &saidaSegura{}
	amb := Ambiente{
		Saida: saida, Getenv: func(k string) string { return env[k] }, Clock: relogio, Versao: "v-test",
		MontarPSPs: func(f Fontes) ports.Registry {
			return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store)))
		},
	}
	ctx, cancelar := context.WithCancel(context.Background())
	fim := make(chan error, 1)
	go func() { fim <- Run(ctx, []string{"sandbox"}, amb) }()
	parar = func() {
		cancelar()
		if err := <-fim; err != nil {
			t.Errorf("sandbox terminou com erro: %v", err)
		}
	}
	url = "http://" + addr
	for range 200 {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url+"/health", nil)
		if r, err := http.DefaultClient.Do(req); err == nil {
			_ = r.Body.Close()
			m := reToken.FindStringSubmatch(saida.String())
			if m == nil {
				parar()
				t.Fatalf("token não impresso:\n%s", saida)
			}
			return url, m[1], parar
		}
		select {
		case err := <-fim:
			t.Fatalf("sandbox não subiu: %v\n%s", err, saida)
		case <-time.After(25 * time.Millisecond):
		}
	}
	parar()
	t.Fatalf("sandbox não respondeu em %s", addr)
	return "", "", nil
}

func chamar(t *testing.T, metodo, url, token, corpo string) map[string]any {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), metodo, url, strings.NewReader(corpo))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	if metodo == http.MethodPost {
		req.Header.Set("Idempotency-Key", "pedido-1")
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	var v map[string]any
	_ = json.NewDecoder(r.Body).Decode(&v)
	if r.StatusCode >= 300 {
		t.Fatalf("%s %s: %d %v", metodo, url, r.StatusCode, v)
	}
	return v
}

// TestSandboxDoZeroAUmaCobrancaPaga segue o banner do `lastro sandbox` (#20) e
// confere que, depois de reiniciar, a chave mestra e o token antigo continuam valendo.
func TestSandboxDoZeroAUmaCobrancaPaga(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "lastro-sandbox.db")
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))

	url, token, parar := subirSandbox(t, db, relogio)
	cob := chamar(t, http.MethodPost, url+"/cob", token, `{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@lastro.dev"}`)
	txid, _ := cob["txid"].(string)
	if cob["status"] != "ATIVA" || txid == "" {
		t.Fatalf("cobrança criada: %v", cob)
	}
	relogio.Avancar(sandbox.AtrasoPadrao + time.Second)
	if lida := chamar(t, http.MethodGet, url+"/cob/"+txid, token, ""); lida["status"] != "CONCLUIDA" {
		t.Fatalf("cobrança depois do atraso: %v", lida)
	}
	parar()

	info, err := os.Stat(filepath.Join(dir, "lastro-sandbox.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissão da chave: %v", info.Mode().Perm())
	}

	// Segunda subida: mesma chave (os dados decifram), conta não duplicada,
	// token novo e o antigo ainda válido.
	url, novo, parar := subirSandbox(t, db, relogio)
	defer parar()
	if novo == token {
		t.Fatal("a segunda subida deveria imprimir uma API key nova")
	}
	if lida := chamar(t, http.MethodGet, url+"/cob/"+txid, token, ""); lida["status"] != "CONCLUIDA" {
		t.Fatalf("depois do restart: %v", lida)
	}
}

func TestSandboxRecusaBancoComContaReal(t *testing.T) {
	env := envComBanco(t)
	amb, _ := ambiente(t, env)
	ctx := context.Background()
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		t.Fatal(err)
	}
	err = contas.Store.Contas().Criar(ctx, domain.Conta{ID: "loja", Nome: "Loja", PSP: "efi", ChavePix: "loja@example.com", Ambiente: domain.Producao, Ativa: true})
	fechar()
	if err != nil {
		t.Fatal(err)
	}
	err = Run(ctx, []string{"sandbox"}, amb)
	if err == nil || !strings.Contains(err.Error(), "sem contas reais") {
		t.Fatalf("esperava recusa, veio %v", err)
	}
}

func TestSandboxGuardaChaveAoLadoDoBanco(t *testing.T) {
	db := filepath.Join(t.TempDir(), "teste.db")
	amb, _ := ambiente(t, map[string]string{})
	chave, arquivo, err := chaveMestraSandbox(amb, db)
	if err != nil {
		t.Fatal(err)
	}
	if arquivo != strings.TrimSuffix(db, ".db")+".key" {
		t.Fatalf("arquivo: %s", arquivo)
	}
	if _, err := vault.New(chave); err != nil {
		t.Fatalf("chave gerada inválida: %v", err)
	}
	outra, _, err := chaveMestraSandbox(amb, db)
	if err != nil || outra != chave {
		t.Fatalf("segunda leitura: %q, %v", outra, err)
	}
	// LASTRO_MASTER_KEY definida vence o arquivo.
	amb.Getenv = func(k string) string { return map[string]string{"LASTRO_MASTER_KEY": "x"}[k] }
	if c, a, _ := chaveMestraSandbox(amb, db); c != "x" || a != "" {
		t.Fatalf("com LASTRO_MASTER_KEY: %q %q", c, a)
	}
}

func TestURLLocal(t *testing.T) {
	for addr, quer := range map[string]string{
		":8080":          "http://localhost:8080",
		"0.0.0.0:9000":   "http://localhost:9000",
		"127.0.0.1:8080": "http://127.0.0.1:8080",
	} {
		if got := urlLocal(addr); got != quer {
			t.Errorf("urlLocal(%q) = %q, quer %q", addr, got, quer)
		}
	}
}
