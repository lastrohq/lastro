package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
)

func carregarContrato(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	spec, err := gen.GetSpec()
	if err != nil {
		t.Fatalf("spec embutida não carrega: %v", err)
	}
	if err := spec.Validate(context.Background()); err != nil {
		t.Fatalf("api/openapi.yaml é inválido: %v", err)
	}
	spec.Servers = nil // casa rotas sem depender do host
	r, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	return spec, r
}

// conferirResposta faz a requisição no roteador real e valida a resposta contra o contrato.
func conferirResposta(t *testing.T, r routers.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	NewRouter("v0.0.0-test").ServeHTTP(rec, req)

	rota, params, err := r.FindRoute(req)
	if err != nil {
		t.Fatalf("%s %s não está no contrato: %v", req.Method, req.URL.Path, err)
	}
	in := &openapi3filter.RequestValidationInput{
		Request: req, PathParams: params, Route: rota,
		Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}
	out := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: in,
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
	}
	if err := openapi3filter.ValidateResponse(req.Context(), out); err != nil {
		t.Fatalf("resposta fora do contrato (%d %s): %v", rec.Code, rec.Body.String(), err)
	}
	return rec
}

func TestContratoEValido(t *testing.T) {
	carregarContrato(t)
}

func TestHealthSegueOContrato(t *testing.T) {
	_, r := carregarContrato(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	if rec := conferirResposta(t, r, req); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRotaPendenteResponde501ComProblema(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/cob/abcdefghijklmnopqrstuvwxyz", nil)
	NewRouter("dev").ServeHTTP(rec, req)
	if rec.Code != http.StatusNotImplemented || rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("status=%d content-type=%q body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
}
