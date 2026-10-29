// Package httpapi expõe a API compatível com a spec do BCB (api/openapi.yaml),
// a API administrativa e os webhooks de entrada.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/ports"
)

// errNaoImplementado marca rotas do contrato que ainda não têm caso de uso.
var errNaoImplementado = errors.New("rota ainda não implementada")

// Opcoes são as dependências do servidor HTTP.
type Opcoes struct {
	Versao    string
	Cobrancas app.Cobrancas
	Contas    app.Contas
	APIKeys   app.APIKeys
	// Idempotencia vazia (Store nil) desliga o Idempotency-Key.
	Idempotencia app.Idempotencia
	Clock        ports.Clock
}

// Server implementa gen.StrictServerInterface.
type Server struct {
	version   string
	cobrancas app.Cobrancas
	contas    app.Contas
	clock     ports.Clock
}

var _ gen.StrictServerInterface = (*Server)(nil)

// NewRouter monta o roteador HTTP a partir do contrato gerado.
func NewRouter(o Opcoes) http.Handler {
	s := &Server{version: o.Versao, cobrancas: o.Cobrancas, contas: o.Contas, clock: o.Clock}
	strict := gen.NewStrictHandlerWithOptions(s, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			escreverProblema(w, gen.Problema{Status: http.StatusBadRequest, Type: tipoErro("RequisicaoInvalida"), Title: "Requisição inválida", Detail: ptr(err.Error())})
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			escreverProblema(w, problemaDe(r.Context(), err))
		},
	})
	return gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{
		// Rodam por rota, depois do roteamento: r.Pattern já diz qual escopo exigir.
		// O último da lista é o mais externo: autentica primeiro, porque a
		// Idempotency-Key pertence à API key.
		Middlewares: []gen.MiddlewareFunc{idempotente(o.Idempotencia), autenticar(o.APIKeys)},
		// Parâmetro de path, query ou header fora do contrato também responde como Problema.
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			escreverProblema(w, gen.Problema{Status: http.StatusBadRequest, Type: tipoErro("RequisicaoInvalida"), Title: "Requisição inválida", Detail: ptr(err.Error())})
		},
	})
}

// Health responde se a instância está no ar.
func (s *Server) Health(context.Context, gen.HealthRequestObject) (gen.HealthResponseObject, error) {
	return gen.Health200JSONResponse{Status: gen.Ok, Version: s.version}, nil
}

// escreverProblema responde no formato de erro da spec do BCB (RFC 7807).
func escreverProblema(w http.ResponseWriter, p gen.Problema) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(p)
}
