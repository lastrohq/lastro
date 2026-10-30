package efi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/vintemhq/vintem/internal/adapters/psp/pixapi"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/oauth"
	"github.com/vintemhq/vintem/internal/ports"
)

// buscarToken pede o token em POST /oauth/token: Basic auth com client id e
// secret, corpo JSON (o Efí não aceita form) e mTLS.
func buscarToken(mtls *http.Client, base string, cred domain.Credenciais, clock ports.Clock) oauth.Buscar {
	return func(ctx context.Context) (oauth.Token, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/oauth/token", bytes.NewReader([]byte(`{"grant_type":"client_credentials"}`)))
		if err != nil {
			return oauth.Token{}, err
		}
		req.SetBasicAuth(cred.ClientID, cred.ClientSecret)
		req.Header.Set("Content-Type", "application/json")
		resp, err := mtls.Do(req)
		if err != nil {
			return oauth.Token{}, fmt.Errorf("efi: pedindo token: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		corpo, err := io.ReadAll(io.LimitReader(resp.Body, pixapi.LimiteResposta))
		if err != nil {
			return oauth.Token{}, fmt.Errorf("efi: lendo token: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			// A resposta de erro do token não traz segredo, mas também não precisa ir inteira para o log.
			return oauth.Token{}, fmt.Errorf("efi: token recusado com status %d (confira client id, secret e se o certificado é do mesmo ambiente): %w", resp.StatusCode, traduzir(resp.StatusCode, corpo))
		}
		return oauth.LerResposta(bytes.NewReader(corpo), clock.Now())
	}
}

// erroEfi é o corpo de erro do Efí. Alguns endpoints usam o formato de problema
// da spec (title/detail); os dois são aceitos.
type erroEfi struct {
	Nome     string `json:"nome"`
	Mensagem string `json:"mensagem"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

// Os nomes de erro do Efí que viram erros do port. O app deixa passar esses e
// trata qualquer outro como falha do PSP (502).
var errosConhecidos = map[string]error{
	"txid_duplicado":           ports.ErrJaExiste,
	"cobranca_nao_encontrada":  ports.ErrNaoEncontrado,
	"status_cobranca_invalido": domain.ErrTransicaoInvalida,
	"pix_nao_encontrado":       ports.ErrNaoEncontrado,
	"devolucao_id_duplicado":   pixapi.ErrDevolucaoDuplicada,
	// Devolução inexistente vem como 400, não 404 (visto na homologação em 29/10/2026).
	"devolucao_nao_encontrada": ports.ErrNaoEncontrado,
}

// ErrEfi é a falha do Efí que não corresponde a um erro conhecido do port.
var ErrEfi = errors.New("o Efí recusou")

func traduzir(status int, corpo []byte) error {
	var e erroEfi
	_ = json.Unmarshal(corpo, &e) // corpo fora do formato: fica só o status
	nome, mensagem := e.Nome, e.Mensagem
	if nome == "" {
		nome, mensagem = e.Title, e.Detail
	}
	if conhecido, ok := errosConhecidos[nome]; ok {
		return fmt.Errorf("%w: %s (%s)", conhecido, mensagem, nome)
	}
	if status == http.StatusNotFound {
		return fmt.Errorf("%w: status 404 %s", ports.ErrNaoEncontrado, nome)
	}
	if nome == "" {
		return fmt.Errorf("%w: status %d", ErrEfi, status)
	}
	return fmt.Errorf("%w: status %d, %s: %s", ErrEfi, status, nome, mensagem)
}
