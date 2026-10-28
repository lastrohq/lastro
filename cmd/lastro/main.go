// Command lastro é o binário único do Lastro: API, painel, workers e MCP.
package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/httpapi"
)

// version é preenchida no build pelo goreleaser (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "version":
		fmt.Println(version)
		return nil
	case "serve":
		return serve()
	default:
		return fmt.Errorf("comando desconhecido %q (use: serve, version)", cmd)
	}
}

func serve() error {
	addr := os.Getenv("LASTRO_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(version),
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("lastro no ar", "addr", addr, "version", version)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
