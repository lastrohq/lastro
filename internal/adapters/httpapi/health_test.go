package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthRespondeOkComVersao(t *testing.T) {
	rec := httptest.NewRecorder()
	NewRouter("v0.0.0-test").ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("corpo não é JSON: %v", err)
	}
	if body["status"] != "ok" || body["version"] != "v0.0.0-test" {
		t.Fatalf("corpo inesperado: %v", body)
	}
}
