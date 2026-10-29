// Package efitest é um Efí de mentira para testes: TLS que exige o certificado
// do cliente, token OAuth2 por Basic auth e corpo JSON, e cob em memória. As
// respostas seguem os exemplos da documentação do Efí, inclusive os desvios
// (erro {"nome","mensagem"} e expiracao como string no GET). Não serve para produção.
package efitest

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lastrohq/lastro/pkg/brcode"
)

// Servidor é o Efí falso.
type Servidor struct {
	URL string
	// Raizes confia no certificado TLS do servidor (passe em efi.ComRaizes).
	Raizes *x509.CertPool

	clientID, clientSecret string

	mu             sync.Mutex
	tokens         map[string]bool
	seq            int
	cobs           map[string]map[string]any
	tokensEmitidos int
	chamadas       []string
	falhas         []int // status forçados para as próximas chamadas à API (não ao token)
}

// Novo sobe o servidor. Só aceita conexões com um certificado de cliente
// emitido por clienteCA, e tokens pedidos com clientID e clientSecret.
func Novo(t testing.TB, clienteCA *x509.Certificate, clientID, clientSecret string) *Servidor {
	t.Helper()
	s := &Servidor{clientID: clientID, clientSecret: clientSecret, tokens: map[string]bool{}, cobs: map[string]map[string]any{}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(s.servir))
	pool := x509.NewCertPool()
	pool.AddCert(clienteCA)
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool, MinVersion: tls.VersionTLS12}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	s.Raizes = x509.NewCertPool()
	s.Raizes.AddCert(srv.Certificate())
	return s
}

// TokensEmitidos conta quantos tokens o servidor entregou.
func (s *Servidor) TokensEmitidos() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokensEmitidos
}

// Chamadas lista "MÉTODO caminho" de cada chamada à API, em ordem.
func (s *Servidor) Chamadas() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.chamadas...)
}

// RevogarTokens invalida os tokens emitidos: a próxima chamada recebe 401.
func (s *Servidor) RevogarTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = map[string]bool{}
}

// Falhar faz as próximas chamadas à API responderem os status dados, em ordem.
func (s *Servidor) Falhar(status ...int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.falhas = append(s.falhas, status...)
}

// Pagar marca a cob como CONCLUIDA, como na homologação do Efí.
func (s *Servidor) Pagar(txid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.cobs[txid]; c != nil {
		c["status"] = "CONCLUIDA"
	}
}

func erro(w http.ResponseWriter, status int, nome, mensagem string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"nome": nome, "mensagem": mensagem})
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}

func (s *Servidor) servir(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/oauth/token" {
		s.token(w, r)
		return
	}
	s.mu.Lock()
	s.chamadas = append(s.chamadas, r.Method+" "+r.URL.Path)
	var forcado int
	if len(s.falhas) > 0 {
		forcado, s.falhas = s.falhas[0], s.falhas[1:]
	}
	autorizado := s.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	s.mu.Unlock()

	switch {
	case !autorizado:
		erro(w, http.StatusUnauthorized, "nao_autorizado", "token ausente, expirado ou revogado")
		return
	case forcado != 0:
		erro(w, forcado, "erro_aplicacao", "falha simulada")
		return
	}
	if txid, ok := strings.CutPrefix(r.URL.Path, "/v2/cob/"); ok {
		s.cob(w, r, txid)
		return
	}
	erro(w, http.StatusNotFound, "rota_nao_encontrada", "rota não existe no Efí falso")
}

func (s *Servidor) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	var corpo struct {
		GrantType string `json:"grant_type"`
	}
	switch {
	case r.Method != http.MethodPost:
		erro(w, http.StatusMethodNotAllowed, "metodo", "use POST")
	case !ok || id != s.clientID || secret != s.clientSecret:
		erro(w, http.StatusUnauthorized, "invalid_client", "credenciais inválidas")
	case r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&corpo) != nil || corpo.GrantType != "client_credentials":
		erro(w, http.StatusBadRequest, "invalid_request", `corpo deve ser JSON {"grant_type":"client_credentials"}`)
	default:
		s.mu.Lock()
		s.seq++
		s.tokensEmitidos++
		token := fmt.Sprintf("token-efi-%d", s.seq)
		s.tokens[token] = true
		s.mu.Unlock()
		responder(w, http.StatusOK, map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3600, "scope": "cob.write cob.read"})
	}
}

func (s *Servidor) cob(w http.ResponseWriter, r *http.Request, txid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	atual := s.cobs[txid]
	switch r.Method {
	case http.MethodPut:
		if atual != nil {
			erro(w, http.StatusConflict, "txid_duplicado", "Campo txid informado já foi utilizado em outra cobrança")
			return
		}
		var corpo map[string]any
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &corpo) != nil {
			erro(w, http.StatusBadRequest, "json_invalido", "corpo não é JSON")
			return
		}
		s.seq++
		location := fmt.Sprintf("pix-h.example.efipay.com.br/v2/%d", s.seq)
		corpo["txid"], corpo["status"], corpo["revisao"], corpo["location"] = txid, "ATIVA", 0, location
		corpo["loc"] = map[string]any{"id": s.seq, "location": location, "tipoCob": "cob"}
		cal, _ := corpo["calendario"].(map[string]any)
		if cal == nil {
			cal = map[string]any{"expiracao": 3600}
		}
		cal["criacao"] = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
		corpo["calendario"] = cal
		corpo["pixCopiaECola"] = brCodeDinamico(location)
		s.cobs[txid] = corpo
		responder(w, http.StatusCreated, corpo)
	case http.MethodGet:
		if atual == nil {
			erro(w, http.StatusNotFound, "cobranca_nao_encontrada", "Nenhuma cobrança encontrada para o txid informado")
			return
		}
		// Desvio documentado: no GET, expiracao vem como string.
		copia := map[string]any{}
		for k, v := range atual {
			copia[k] = v
		}
		if cal, ok := atual["calendario"].(map[string]any); ok {
			c := map[string]any{"criacao": cal["criacao"], "expiracao": fmt.Sprint(cal["expiracao"])}
			copia["calendario"] = c
		}
		responder(w, http.StatusOK, copia)
	case http.MethodPatch:
		if atual == nil {
			erro(w, http.StatusNotFound, "cobranca_nao_encontrada", "Nenhuma cobrança encontrada para o txid informado")
			return
		}
		if atual["status"] != "ATIVA" {
			erro(w, http.StatusBadRequest, "status_cobranca_invalido", "A cobrança não está mais com o status ATIVA")
			return
		}
		var mudancas map[string]any
		if b, _ := io.ReadAll(r.Body); json.Unmarshal(b, &mudancas) != nil {
			erro(w, http.StatusBadRequest, "json_invalido", "corpo não é JSON")
			return
		}
		for k, v := range mudancas {
			if k == "calendario" {
				cal, _ := atual["calendario"].(map[string]any)
				for ck, cv := range v.(map[string]any) {
					cal[ck] = cv
				}
				continue
			}
			atual[k] = v
		}
		atual["revisao"] = atual["revisao"].(int) + 1
		responder(w, http.StatusOK, atual)
	default:
		erro(w, http.StatusMethodNotAllowed, "metodo", "método não suportado")
	}
}

// brCodeDinamico gera o "Pix Copia e Cola" que o Efí devolveria para a location.
func brCodeDinamico(location string) string {
	code, err := brcode.Encode(brcode.Pix{URL: location, Nome: "EFI TESTE", Cidade: "SAO PAULO"})
	if err != nil {
		panic(err) // location gerada aqui mesmo: só falha se o código estiver errado
	}
	return code
}
