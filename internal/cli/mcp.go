package cli

import (
	"context"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/lastrohq/lastro/internal/adapters/mcp"
)

// mcpCmd roda o servidor MCP pela entrada e saída padrão (#38). Ele fala com a
// API HTTP do Lastro, então os escopos da API key valem como para qualquer
// cliente (ADR-008). Nada pode ir para a saída padrão além do protocolo.
func mcpCmd(ctx context.Context, amb Ambiente) error {
	api := mcpAPI(amb)
	if api.Token == "" {
		return mcp.ErrSemToken
	}
	return mcp.NovoServidor(api, amb.Versao).Run(ctx, &sdk.StdioTransport{})
}

func mcpAPI(amb Ambiente) mcp.API {
	return mcp.API{
		URL:      getenv(amb, "LASTRO_URL", urlLocal(getenv(amb, "LASTRO_ADDR", ":8080"))),
		Token:    amb.Getenv("LASTRO_TOKEN"),
		ChavePix: amb.Getenv("LASTRO_CHAVE_PIX"),
	}
}
