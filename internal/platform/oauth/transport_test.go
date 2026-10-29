package oauth_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/platform/oauth"
)

// api simula a API Pix do banco: só aceita o token da lista e guarda o que recebeu.
type api struct {
	aceitos  map[string]bool
	chamadas atomic.Int64

	mu     sync.Mutex
	corpos []string
	auths  []string
}

func (a *api) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.chamadas.Add(1)
	corpo, _ := io.ReadAll(r.Body)
	a.mu.Lock()
	a.corpos = append(a.corpos, string(corpo))
	a.auths = append(a.auths, r.Header.Get("Authorization"))
	a.mu.Unlock()
	if !a.aceitos[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *api) recebido(i int) (corpo, auth string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.corpos[i], a.auths[i]
}

// pedir faz a requisição com o contexto do teste.
func pedir(t *testing.T, cli *http.Client, metodo, url string, corpo io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), metodo, url, corpo)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp
}

// cliente sobe a API falsa e devolve um http.Client com o Transport do cache.
func cliente(t *testing.T, aceitos ...string) (*http.Client, *api, *banco, string) {
	t.Helper()
	a := &api{aceitos: map[string]bool{}}
	for _, v := range aceitos {
		a.aceitos[v] = true
	}
	srv := httptest.NewServer(a)
	t.Cleanup(srv.Close)
	b, c := novoBanco(time.Hour)
	return &http.Client{Transport: &oauth.Transport{Cache: c, Chave: "loja", Buscar: b.buscar}}, a, b, srv.URL
}

func TestTransportEnviaOToken(t *testing.T) {
	cli, a, _, url := cliente(t, "token-1")
	resp := pedir(t, cli, http.MethodGet, url, nil)
	if _, auth := a.recebido(0); resp.StatusCode != http.StatusOK || auth != "Bearer token-1" {
		t.Fatalf("status %d, Authorization %q", resp.StatusCode, auth)
	}
}

func Test401InvalidaOTokenETentaUmaVez(t *testing.T) {
	// O banco revogou o token-1 antes de ele expirar: só aceita o token-2.
	cli, a, b, url := cliente(t, "token-2")

	resp := pedir(t, cli, http.MethodPost, url, strings.NewReader(`{"valor":"1.00"}`))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d; quer 200 na segunda tentativa", resp.StatusCode)
	}
	if n := b.buscas.Load(); n != 2 {
		t.Fatalf("buscas de token = %d; quer 2", n)
	}
	if n := a.chamadas.Load(); n != 2 {
		t.Fatalf("chamadas à API = %d; quer 2", n)
	}
	if corpo, _ := a.recebido(1); corpo != `{"valor":"1.00"}` {
		t.Fatalf("a segunda tentativa mandou o corpo %q; quer o mesmo da primeira", corpo)
	}

	// O token novo fica no cache para as próximas chamadas.
	pedir(t, cli, http.MethodGet, url, nil)
	if n := b.buscas.Load(); n != 2 {
		t.Fatalf("buscas = %d depois de renovar; quer 2", n)
	}
}

func Test401DuasVezesNaoEntraEmLaco(t *testing.T) {
	cli, a, b, url := cliente(t) // nenhum token é aceito

	resp := pedir(t, cli, http.MethodGet, url, nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d; quer o 401 do banco", resp.StatusCode)
	}
	if n := a.chamadas.Load(); n != 2 {
		t.Fatalf("chamadas = %d; quer 2 (uma tentativa extra, não mais)", n)
	}
	if n := b.buscas.Load(); n != 2 {
		t.Fatalf("buscas = %d; quer 2", n)
	}
}

func TestCorpoSemGetBodyNaoRepete(t *testing.T) {
	cli, a, _, url := cliente(t, "token-2")

	// io.MultiReader esconde o tipo: a requisição fica sem GetBody.
	resp := pedir(t, cli, http.MethodPost, url, io.MultiReader(bytes.NewReader([]byte("x"))))
	if resp.StatusCode != http.StatusUnauthorized || a.chamadas.Load() != 1 {
		t.Fatalf("status %d em %d chamadas; quer o 401 sem repetir", resp.StatusCode, a.chamadas.Load())
	}
}

func TestTransportNaoAlteraARequisicaoOriginal(t *testing.T) {
	cli, _, _, url := cliente(t, "token-1")
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	resp, err := cli.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if h := req.Header.Get("Authorization"); h != "" {
		t.Fatalf("a requisição original ganhou Authorization %q", h)
	}
}
