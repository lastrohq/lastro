package efi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/oauth"
	"github.com/lastrohq/lastro/internal/ports"
)

// limiteResposta protege contra resposta gigante (bug ou ataque no caminho).
const limiteResposta = 4 << 20

// buscarToken pede o token em POST /oauth/token: Basic auth com client id e
// secret, corpo JSON (o Efí não aceita form) e mTLS.
func buscarToken(mtls *http.Client, base, clientID, clientSecret string, clock ports.Clock) oauth.Buscar {
	return func(ctx context.Context) (oauth.Token, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/oauth/token", bytes.NewReader([]byte(`{"grant_type":"client_credentials"}`)))
		if err != nil {
			return oauth.Token{}, err
		}
		req.SetBasicAuth(clientID, clientSecret)
		req.Header.Set("Content-Type", "application/json")
		resp, err := mtls.Do(req)
		if err != nil {
			return oauth.Token{}, fmt.Errorf("efi: pedindo token: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		corpo, err := io.ReadAll(io.LimitReader(resp.Body, limiteResposta))
		if err != nil {
			return oauth.Token{}, fmt.Errorf("efi: lendo token: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			// A resposta de erro do token não traz segredo, mas também não precisa ir inteira para o log.
			return oauth.Token{}, fmt.Errorf("efi: token recusado com status %d (confira client id, secret e se o certificado é do mesmo ambiente): %w", resp.StatusCode, erroDo(resp.StatusCode, corpo))
		}
		return oauth.LerResposta(bytes.NewReader(corpo), clock.Now())
	}
}

// chamar faz a requisição na API Pix da conta e decodifica a resposta em saida.
func (p *PSP) chamar(ctx context.Context, conta domain.Conta, metodo, caminho string, entrada, saida any) error {
	return p.chamarCom(ctx, conta, metodo, caminho, entrada, saida, nil)
}

// chamarCom é o chamar com headers extras.
func (p *PSP) chamarCom(ctx context.Context, conta domain.Conta, metodo, caminho string, entrada, saida any, headers map[string]string) error {
	c, err := p.clienteDa(ctx, conta)
	if err != nil {
		return err
	}
	var corpo io.Reader
	if entrada != nil {
		b, err := json.Marshal(entrada)
		if err != nil {
			return fmt.Errorf("efi: montando %s %s: %w", metodo, caminho, err)
		}
		// bytes.Reader permite ao oauth.Transport reenviar o corpo depois de um 401.
		corpo = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, c.base+caminho, corpo)
	if err != nil {
		return err
	}
	if entrada != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("efi: %s %s: %w", metodo, caminho, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, limiteResposta))
	if err != nil {
		return fmt.Errorf("efi: lendo resposta de %s %s: %w", metodo, caminho, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("efi: %s %s: %w", metodo, caminho, erroDo(resp.StatusCode, b))
	}
	if saida == nil {
		return nil
	}
	if err := json.Unmarshal(b, saida); err != nil {
		return fmt.Errorf("efi: resposta de %s %s fora do formato: %w", metodo, caminho, err)
	}
	return nil
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
	"devolucao_id_duplicado":   errDevolucaoDuplicada,
}

// errDevolucaoDuplicada não sai do adapter: o Devolver a transforma em idempotência.
var errDevolucaoDuplicada = errors.New("devolução com esse id já existe")

// ErrEfi é a falha do Efí que não corresponde a um erro conhecido do port.
var ErrEfi = errors.New("o Efí recusou")

// erroHTTP guarda o status da resposta de erro, para quem precisa decidir por ele.
type erroHTTP struct {
	status int
	err    error
}

func (e *erroHTTP) Error() string { return e.err.Error() }
func (e *erroHTTP) Unwrap() error { return e.err }

func erroDo(status int, corpo []byte) error {
	return &erroHTTP{status: status, err: traduzir(status, corpo)}
}

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
