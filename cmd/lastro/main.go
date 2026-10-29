// Command lastro é o binário único do Lastro: API, painel, workers e MCP.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/cli"
	"github.com/lastrohq/lastro/internal/platform/clock"
)

// version é preenchida no build pelo goreleaser (-ldflags "-X main.version=...").
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	relogio := clock.Sistema{}
	err := cli.Run(ctx, os.Args[1:], cli.Ambiente{
		Saida:  os.Stdout,
		Getenv: os.Getenv,
		PSPs:   psp.NewRegistry(sandbox.New(relogio)),
		Clock:  relogio,
		Versao: version,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
