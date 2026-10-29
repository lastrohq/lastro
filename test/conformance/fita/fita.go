// Package fita grava as chamadas HTTP de um adapter contra o banco de verdade
// e as reproduz depois, sem rede (RN-51). Cada teste tem a sua fita, um JSON em
// test/fixtures/<banco>/. Na reprodução o adapter precisa fazer as mesmas
// chamadas, na mesma ordem, com os mesmos corpos; qualquer desvio falha o teste.
//
// A fita não guarda cabeçalhos (lá vão o Basic do client secret e o Bearer),
// troca o access_token das respostas de OAuth por "gravado" e troca os dados
// de pessoas (recebedor e pagador, com nome, CPF e endereço) por fictícios.
package fita

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Fita é a gravação de um teste.
type Fita struct {
	// Agora é o instante da gravação. A suíte usa ele como relógio nos dois
	// modos, para que os corpos e as janelas de consulta saiam iguais.
	Agora time.Time `json:"agora"`
	// Rodada distingue os txids de uma gravação dos da anterior.
	Rodada string `json:"rodada"`
	// Dados guarda o que o teste precisa repetir na reprodução, como a chave
	// Pix da conta usada na gravação.
	Dados      map[string]string `json:"dados,omitempty"`
	Interacoes []Interacao       `json:"interacoes"`

	t       testing.TB
	arquivo string
	gravar  bool
	mu      sync.Mutex
	proxima int
}

// Interacao é uma chamada e a resposta do banco.
type Interacao struct {
	Metodo   string          `json:"metodo"`
	URL      string          `json:"url"` // caminho e query, sem o host
	Corpo    json.RawMessage `json:"corpo,omitempty"`
	Status   int             `json:"status"`
	Resposta json.RawMessage `json:"resposta,omitempty"`
}

// Abrir prepara a fita do teste. Gravando, começa uma fita nova e a salva no
// fim do teste se ele passar. Reproduzindo, lê o arquivo; sem arquivo, a fita
// fica vazia e só serve para teste que não chama o banco.
func Abrir(t testing.TB, arquivo string, gravar bool) *Fita {
	t.Helper()
	f := &Fita{t: t, arquivo: arquivo, gravar: gravar}
	if gravar {
		f.Agora = time.Now().UTC().Truncate(time.Second)
		f.Rodada = f.Agora.Format("0102150405")[:8]
		t.Cleanup(f.salvar)
		return f
	}
	b, err := os.ReadFile(arquivo) //nolint:gosec // G304: caminho montado pelo teste
	switch {
	case errors.Is(err, fs.ErrNotExist):
		f.Agora = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	case err != nil:
		t.Fatal(err)
	default:
		if err := json.Unmarshal(b, f); err != nil {
			t.Fatalf("fita %s: %v", arquivo, err)
		}
	}
	t.Cleanup(func() {
		if !t.Failed() && f.proxima < len(f.Interacoes) {
			t.Errorf("fita %s: o adapter fez %d chamadas, a gravação tem %d", arquivo, f.proxima, len(f.Interacoes))
		}
	})
	return f
}

// Arquivo devolve o caminho da fita de um teste: o nome (em geral t.Name())
// vira o nome do arquivo, com o que não for letra, número ou hífen trocado por "_".
func Arquivo(dir, nome string) string {
	nome = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		}
		return '_'
	}, nome)
	return filepath.Join(dir, nome+".json")
}

// Envolver devolve o transporte que grava (passando para base) ou reproduz.
func (f *Fita) Envolver(base http.RoundTripper) http.RoundTripper {
	return transporte{f: f, base: base}
}

type transporte struct {
	f    *Fita
	base http.RoundTripper
}

func (tr transporte) RoundTrip(req *http.Request) (*http.Response, error) {
	corpo, err := lerCorpo(req)
	if err != nil {
		return nil, err
	}
	pedido := Interacao{Metodo: req.Method, URL: req.URL.RequestURI(), Corpo: jsonOuNada(corpo)}
	if tr.f.gravar {
		return tr.gravar(req, pedido)
	}
	return tr.f.reproduzir(req, pedido)
}

func (tr transporte) gravar(req *http.Request, pedido Interacao) (*http.Response, error) {
	resp, err := tr.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	pedido.Status = resp.StatusCode
	pedido.Resposta = jsonOuNada(mascarar(semToken(b)))
	tr.f.mu.Lock()
	tr.f.Interacoes = append(tr.f.Interacoes, pedido)
	tr.f.mu.Unlock()
	return resp, nil
}

func (f *Fita) reproduzir(req *http.Request, pedido Interacao) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.proxima >= len(f.Interacoes) {
		f.t.Errorf("fita %s: chamada que não está na gravação (grave de novo): %s %s", f.arquivo, pedido.Metodo, pedido.URL)
		return nil, fmt.Errorf("fita: chamada não gravada: %s %s", pedido.Metodo, pedido.URL)
	}
	g := f.Interacoes[f.proxima]
	f.proxima++
	if g.Metodo != pedido.Metodo || g.URL != pedido.URL || !mesmoJSON(g.Corpo, pedido.Corpo) {
		f.t.Errorf("fita %s, chamada %d:\n gravada: %s %s %s\n feita:   %s %s %s",
			f.arquivo, f.proxima, g.Metodo, g.URL, g.Corpo, pedido.Metodo, pedido.URL, pedido.Corpo)
		return nil, errors.New("fita: chamada diferente da gravada")
	}
	return &http.Response{
		StatusCode: g.Status,
		Status:     fmt.Sprintf("%d %s", g.Status, http.StatusText(g.Status)),
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(g.Resposta)),
		Request:    req,
	}, nil
}

func (f *Fita) salvar() {
	if len(f.Interacoes) == 0 {
		return
	}
	arquivo := f.arquivo
	if f.t.Failed() {
		// Fica para diagnóstico, fora das fitas: o .gitignore exclui *.falha.
		arquivo += ".falha"
		f.t.Logf("teste falhou: a gravação ficou em %s, só para diagnóstico", arquivo)
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		f.t.Error(err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(arquivo), 0o750); err != nil {
		f.t.Error(err)
		return
	}
	if err := os.WriteFile(arquivo, append(b, '\n'), 0o600); err != nil {
		f.t.Error(err)
	}
}

func lerCorpo(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	b, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}

// jsonOuNada guarda o corpo como JSON; o que não for JSON vira uma string JSON.
func jsonOuNada(b []byte) json.RawMessage {
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if json.Valid(b) {
		var compacto bytes.Buffer
		if json.Compact(&compacto, b) == nil {
			return compacto.Bytes()
		}
	}
	s, _ := json.Marshal(string(b))
	return s
}

func mesmoJSON(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return bytes.Equal(a, b)
	}
	ja, _ := json.Marshal(va)
	jb, _ := json.Marshal(vb)
	return bytes.Equal(ja, jb)
}

// semToken troca o access_token de uma resposta de OAuth por "gravado".
func semToken(b []byte) []byte {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return b
	}
	if _, ok := m["access_token"]; !ok {
		return b
	}
	m["access_token"] = "gravado"
	out, err := json.Marshal(m)
	if err != nil {
		return b
	}
	return out
}

// Pessoas fictícias que entram no lugar das de verdade. A forma (objeto com os
// campos da spec) é mantida para o adapter conseguir ler a resposta.
var pessoasFicticias = map[string]map[string]string{
	"recebedor": {"nome": "Recebedor de Teste", "cpf": "00000000000", "logradouro": "Rua de Teste, 1", "cidade": "Sao Paulo", "uf": "SP", "cep": "01000000"},
	"pagador":   {"nome": "Pagador de Teste", "cpf": "00000000000"},
}

// mascarar troca, em qualquer nível do JSON, os objetos de pessoas pelos
// fictícios; só os campos que vieram são trocados, e CNPJ vira CNPJ.
func mascarar(b []byte) []byte {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return b
	}
	out, err := json.Marshal(trocarPessoas(v))
	if err != nil {
		return b
	}
	return out
}

func trocarPessoas(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, filho := range x {
			if ficticia, ok := pessoasFicticias[k]; ok {
				if pessoa, ok := filho.(map[string]any); ok {
					for campo := range pessoa {
						switch {
						case campo == "cnpj":
							pessoa[campo] = "00000000000000"
						case ficticia[campo] != "":
							pessoa[campo] = ficticia[campo]
						default:
							pessoa[campo] = "mascarado"
						}
					}
					continue
				}
			}
			x[k] = trocarPessoas(filho)
		}
	case []any:
		for i := range x {
			x[i] = trocarPessoas(x[i])
		}
	}
	return v
}
