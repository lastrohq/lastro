package httpapi

import (
	"encoding/json"
	"net/http"
)

// NewRouter monta o roteador HTTP do Lastro.
func NewRouter(version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
	})
	return mux
}
