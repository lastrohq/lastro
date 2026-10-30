// Command lastro é o binário único do Lastro: API, painel, workers e MCP.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/efi"
	"github.com/lastrohq/lastro/internal/adapters/psp/inter"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/adapters/psp/sicoob"
	"github.com/lastrohq/lastro/internal/cli"
	"github.com/lastrohq/lastro/internal/platform/clock"
	"github.com/lastrohq/lastro/internal/ports"
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
		// Os bancos reais precisam das credenciais das contas, e o sandbox do
		// store, que só existem depois de abrir o store e o vault: a CLI monta o
		// registry nessa hora.
		MontarPSPs: func(f cli.Fontes) ports.Registry {
			return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store)), efi.New(f.Credenciais, relogio), inter.New(f.Credenciais, relogio), sicoob.New(f.Credenciais, relogio))
		},
		Clock:  relogio,
		Versao: version,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
