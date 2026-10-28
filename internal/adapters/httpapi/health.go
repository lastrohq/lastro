// Package httpapi expõe a API compatível com a spec do BCB (api/openapi.yaml),
// a API administrativa e os webhooks de entrada.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
)

// errNaoImplementado marca rotas do contrato que ainda não têm caso de uso.
var errNaoImplementado = errors.New("rota ainda não implementada")

// Server implementa gen.StrictServerInterface.
type Server struct {
	version string
}

var _ gen.StrictServerInterface = (*Server)(nil)

// NewRouter monta o roteador HTTP a partir do contrato gerado.
func NewRouter(version string) http.Handler {
	strict := gen.NewStrictHandlerWithOptions(&Server{version: version}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			escreverProblema(w, http.StatusBadRequest, "RequisicaoInvalida", "Requisição inválida", err.Error())
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			if errors.Is(err, errNaoImplementado) {
				escreverProblema(w, http.StatusNotImplemented, "NaoImplementado", "Ainda não implementado", err.Error())
				return
			}
			escreverProblema(w, http.StatusInternalServerError, "ErroInterno", "Erro interno", "")
		},
	})
	return gen.Handler(strict)
}

// Health responde se a instância está no ar.
func (s *Server) Health(context.Context, gen.HealthRequestObject) (gen.HealthResponseObject, error) {
	return gen.Health200JSONResponse{Status: gen.Ok, Version: s.version}, nil
}

// escreverProblema responde no formato de erro da spec do BCB (RFC 7807).
func escreverProblema(w http.ResponseWriter, status int, tipo, titulo, detalhe string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	p := gen.Problema{Type: "https://lastro.dev/erros/" + tipo, Title: titulo, Status: status}
	if detalhe != "" {
		p.Detail = &detalhe
	}
	_ = json.NewEncoder(w).Encode(p)
}
