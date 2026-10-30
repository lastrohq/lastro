package fita

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// espiao guarda os erros em vez de falhar o teste de verdade.
type espiao struct {
	testing.TB
	erros []string
}

func (e *espiao) Errorf(format string, args ...any) {
	e.erros = append(e.erros, fmt.Sprintf(format, args...))
}
func (e *espiao) Failed() bool { return len(e.erros) > 0 }

func banco(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"access_token":"segredo-do-banco","expires_in":3600}`)
			return
		}
		corpo, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"recebido":%s}`, corpo)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func chamar(t *testing.T, c *http.Client, metodo, url, corpo string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), metodo, url, strings.NewReader(corpo))
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("client-id", "client-secret")
	resp, err := c.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func gravar(t *testing.T, arquivo, url string) {
	t.Run("gravar", func(t *testing.T) {
		f := Abrir(t, arquivo, true)
		c := &http.Client{Transport: f.Envolver(http.DefaultTransport)}
		chamar(t, c, http.MethodPost, url+"/oauth/token", "grant_type=client_credentials")
		if st, _ := chamar(t, c, http.MethodPut, url+"/v2/cob/abc?x=1", `{"valor": {"original": "1.00"}}`); st != http.StatusCreated {
			t.Fatalf("status %d", st)
		}
	})
}

func TestGravaSemSegredoEReproduzIgual(t *testing.T) {
	srv := banco(t)
	arquivo := filepath.Join(t.TempDir(), "fita.json")
	gravar(t, arquivo, srv.URL)

	b, err := os.ReadFile(arquivo) //nolint:gosec // G304: arquivo do próprio teste
	if err != nil {
		t.Fatal(err)
	}
	for _, segredo := range []string{"segredo-do-banco", "client-secret", "Basic", "127.0.0.1"} {
		if strings.Contains(string(b), segredo) {
			t.Errorf("a fita guardou %q:\n%s", segredo, b)
		}
	}

	srv.Close()
	f := Abrir(t, arquivo, false)
	c := &http.Client{Transport: f.Envolver(nil)}
	if _, corpo := chamar(t, c, http.MethodPost, "https://outro.host/oauth/token", "grant_type=client_credentials"); !strings.Contains(corpo, `"gravado"`) {
		t.Fatalf("token reproduzido: %s", corpo)
	}
	// JSON com outra formatação é o mesmo corpo.
	st, corpo := chamar(t, c, http.MethodPut, "https://outro.host/v2/cob/abc?x=1", `{"valor":{"original":"1.00"}}`)
	if st != http.StatusCreated || !mesmoJSON([]byte(corpo), []byte(`{"recebido":{"valor":{"original":"1.00"}}}`)) {
		t.Fatalf("reproduzido: %d %s", st, corpo)
	}
}

func TestReproducaoAcusaDesvio(t *testing.T) {
	srv := banco(t)
	arquivo := filepath.Join(t.TempDir(), "fita.json")
	gravar(t, arquivo, srv.URL)

	casos := map[string]func(*testing.T, *http.Client){
		"corpo diferente": func(t *testing.T, c *http.Client) {
			chamar(t, c, http.MethodPost, "http://x/oauth/token", "grant_type=client_credentials")
			chamar(t, c, http.MethodPut, "http://x/v2/cob/abc?x=1", `{"valor":{"original":"2.00"}}`)
		},
		"caminho diferente": func(t *testing.T, c *http.Client) {
			chamar(t, c, http.MethodPost, "http://x/oauth/token", "grant_type=client_credentials")
			chamar(t, c, http.MethodPut, "http://x/v2/cob/outro?x=1", `{"valor":{"original":"1.00"}}`)
		},
		"chamada a mais": func(t *testing.T, c *http.Client) {
			chamar(t, c, http.MethodPost, "http://x/oauth/token", "grant_type=client_credentials")
			chamar(t, c, http.MethodPut, "http://x/v2/cob/abc?x=1", `{"valor":{"original":"1.00"}}`)
			chamar(t, c, http.MethodGet, "http://x/v2/cob/abc", "")
		},
		"chamada a menos": func(t *testing.T, c *http.Client) {
			chamar(t, c, http.MethodPost, "http://x/oauth/token", "grant_type=client_credentials")
		},
	}
	for nome, fazer := range casos {
		t.Run(nome, func(t *testing.T) {
			e := &espiao{TB: t}
			t.Run("reproduzir", func(t *testing.T) {
				e.TB = t
				f := Abrir(e, arquivo, false)
				fazer(t, &http.Client{Transport: f.Envolver(nil)})
			})
			if len(e.erros) == 0 {
				t.Fatal("o desvio passou sem erro")
			}
		})
	}
}

func TestSemFitaNaoGravaNadaENaoDeixaChamar(t *testing.T) {
	dir := t.TempDir()
	t.Run("gravar sem chamadas", func(t *testing.T) { Abrir(t, filepath.Join(dir, "vazia.json"), true) })
	if _, err := os.Stat(filepath.Join(dir, "vazia.json")); err == nil {
		t.Fatal("gravou fita sem chamadas")
	}
	e := &espiao{TB: t}
	f := Abrir(e, filepath.Join(dir, "vazia.json"), false)
	if st, _ := chamar(t, &http.Client{Transport: f.Envolver(nil)}, http.MethodGet, "http://x/v2/cob/abc", ""); st != 0 || len(e.erros) == 0 {
		t.Fatalf("chamada sem fita: status %d, erros %v", st, e.erros)
	}
}

func TestArquivo(t *testing.T) {
	if got := Arquivo("dir", "TestX/cob:_criar_é_ATIVA"); got != filepath.Join("dir", "TestX_cob__criar___ATIVA.json") {
		t.Fatal(got)
	}
}

func TestMascaraPessoas(t *testing.T) {
	corpo := `{"txid":"abc","recebedor":{"nome":"Fulano Real","cpf":"12345678909","logradouro":"Rua Real, 10","cep":"37200000","nomeFantasia":"Loja Real"},
	  "pix":[{"pagador":{"nome":"Beltrana","cnpj":"11222333000181"},"valor":"1.00"}],"devedor":{"nome":"Empresa de Teste"}}`
	got := string(mascarar([]byte(corpo)))
	for _, real := range []string{"Fulano", "12345678909", "Rua Real", "37200000", "Loja Real", "Beltrana", "11222333000181"} {
		if strings.Contains(got, real) {
			t.Errorf("%q ficou na fita: %s", real, got)
		}
	}
	// O devedor é escrito pela suíte (já é fictício) e o resto fica como veio.
	for _, fica := range []string{`"txid":"abc"`, `"valor":"1.00"`, `"Empresa de Teste"`, `"Recebedor de Teste"`, `"00000000000000"`} {
		if !strings.Contains(got, fica) {
			t.Errorf("faltou %s: %s", fica, got)
		}
	}
}

// O Inter manda client_id e client_secret no corpo do formulário do token: a
// fita não pode guardar o segredo, e a reprodução compara com ele trocado.
func TestCredencialNoCorpoNaoVaiParaAFita(t *testing.T) {
	srv := banco(t)
	arquivo := filepath.Join(t.TempDir(), "fita.json")
	t.Run("gravar", func(t *testing.T) {
		f := Abrir(t, arquivo, true)
		c := &http.Client{Transport: f.Envolver(http.DefaultTransport)}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/oauth/token",
			strings.NewReader("client_id=id-real&client_secret=segredo-real&grant_type=client_credentials"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		req, _ = http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/login", strings.NewReader(`{"client_secret":"segredo-json","x":1}`))
		req.Header.Set("Content-Type", "application/json")
		if resp, err = c.Do(req); err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	})
	b, _ := os.ReadFile(arquivo) //nolint:gosec // G304: arquivo do próprio teste
	for _, segredo := range []string{"segredo-real", "id-real", "segredo-json"} {
		if strings.Contains(string(b), segredo) {
			t.Fatalf("a fita guardou %q:\n%s", segredo, b)
		}
	}
	f := Abrir(t, arquivo, false)
	c := &http.Client{Transport: f.Envolver(nil)}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://x/oauth/token",
		strings.NewReader("client_id=outro&client_secret=outro&grant_type=client_credentials"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := c.Do(req); err != nil {
		t.Fatalf("reprodução com outra credencial: %v", err)
	} else {
		_ = resp.Body.Close()
	}
	if st, _ := chamar(t, c, http.MethodPost, "http://x/login", `{"x":1,"client_secret":"outro"}`); st == 0 {
		t.Fatal("reprodução do JSON com outra credencial falhou")
	}
}

func TestChaveNaURLNaoVaiParaAFita(t *testing.T) {
	srv := banco(t)
	arquivo := filepath.Join(t.TempDir(), "fita.json")
	t.Run("gravar", func(t *testing.T) {
		c := &http.Client{Transport: Abrir(t, arquivo, true).Envolver(http.DefaultTransport)}
		if st, _ := chamar(t, c, http.MethodGet, srv.URL+"/pix/v2/cob/abc?gw-dev-app-key=chave-real&x=1", ""); st == 0 {
			t.Fatal("gravação falhou")
		}
	})
	b, _ := os.ReadFile(arquivo) //nolint:gosec // G304: arquivo do próprio teste
	if strings.Contains(string(b), "chave-real") {
		t.Fatalf("a fita guardou a chave de aplicação:\n%s", b)
	}
	c := &http.Client{Transport: Abrir(t, arquivo, false).Envolver(nil)}
	if st, _ := chamar(t, c, http.MethodGet, "http://x/pix/v2/cob/abc?gw-dev-app-key=outra&x=1", ""); st == 0 {
		t.Fatal("reprodução com outra chave falhou")
	}
}
