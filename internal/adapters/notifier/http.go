// Package notifier entrega os webhooks de saída por HTTP (RN-32).
package notifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vintemhq/vintem/internal/ports"
)

// HTTP é o ports.Enviador de verdade.
type HTTP struct {
	cliente *http.Client
	agente  string
}

var _ ports.Enviador = (*HTTP)(nil)

// NovoHTTP cria o enviador. Redirecionamento não é seguido: o destino é a URL
// configurada, e um 3xx conta como falha visível em vez de entrega silenciosa
// em outro lugar.
func NovoHTTP(versao string) *HTTP {
	return &HTTP{
		agente: "Vintém/" + versao,
		cliente: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

// Enviar faz o POST com os cabeçalhos dados.
func (h *HTTP) Enviar(ctx context.Context, url string, cabecalhos map[string]string, corpo []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(corpo))
	if err != nil {
		return 0, fmt.Errorf("montando requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", h.agente)
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}
	resp, err := h.cliente.Do(req)
	if err != nil {
		var urlErr interface{ Timeout() bool }
		if errors.As(err, &urlErr) && urlErr.Timeout() {
			return 0, fmt.Errorf("sem resposta em 10 s")
		}
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10)) // permite reaproveitar a conexão
	return resp.StatusCode, nil
}
