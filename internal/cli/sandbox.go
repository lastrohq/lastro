package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"strings"

	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/platform/vault"
	"github.com/vintemhq/vintem/internal/ports"
)

// Conta que o modo sandbox cria na primeira vez.
const (
	contaSandbox = "sandbox"
	chaveSandbox = "sandbox@vintem.dev"
	urlPadrao    = "http://localhost:8080"
)

// sandboxCmd sobe o Vintém pronto para testar, sem configurar nada (#20): banco
// separado, chave mestra guardada ao lado dele, conta sandbox e uma API key nova
// impressa no terminal junto com um curl que já funciona.
func sandboxCmd(ctx context.Context, amb Ambiente) error {
	db := getenv(amb, "VINTEM_DB", "vintem-sandbox.db")
	chave, arquivoChave, err := chaveMestraSandbox(amb, db)
	if err != nil {
		return err
	}
	getenvOriginal := amb.Getenv
	amb.Getenv = func(k string) string {
		switch k {
		case "VINTEM_DB":
			return db
		case "VINTEM_MASTER_KEY":
			return chave
		case "VINTEM_CONCILIACAO_INTERVALO":
			// O sandbox não custa nada: a conciliação roda a cada 10 s e o
			// pagamento aparece sem ninguém precisar consultar.
			if v := getenvOriginal(k); v != "" {
				return v
			}
			return "10s"
		}
		return getenvOriginal(k)
	}

	token, err := prepararSandbox(ctx, amb, db)
	if err != nil {
		return err
	}
	dados := db
	if arquivoChave != "" {
		dados += " (chave mestra em " + arquivoChave + ")"
	}
	url := urlLocal(getenv(amb, "VINTEM_ADDR", ":8080"))
	// Fora da porta padrão, o MCP precisa da URL: sem ela ele tenta :8080.
	mcp := "-e VINTEM_TOKEN=" + token + " -e VINTEM_CHAVE_PIX=" + chaveSandbox
	if url != urlPadrao {
		mcp += " -e VINTEM_URL=" + url
	}
	_, err = fmt.Fprintf(amb.Saida, `
🧪 Vintém em modo sandbox: nenhum banco e nenhum dinheiro de verdade.

  API       %[1]s
  Painel    %[1]s/painel/ (entre com a API key abaixo)
  Conta     %[2]s (chave Pix %[3]s)
  API key   %[4]s
  Dados     %[5]s

Crie uma cobrança:

  export VINTEM_TOKEN=%[4]s
  curl -s -X POST %[1]s/cob \
    -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: pedido-$RANDOM" \
    -H 'Content-Type: application/json' \
    -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"%[3]s"}'

Em 5 s o "pagador" paga. Consulte com GET %[1]s/cob/{txid} e veja CONCLUIDA.

Use com seu agente (Claude Code):

  claude mcp add vintem %[6]s -- vintem mcp

Valores mágicos e o que o sandbox simula: docs/SANDBOX.md

`, url, contaSandbox, chaveSandbox, token, dados, mcp)
	if err != nil {
		return err
	}
	return serve(ctx, amb)
}

// chaveMestraSandbox usa VINTEM_MASTER_KEY se definida; senão lê (ou cria) o
// arquivo de chave ao lado do banco, para os dados sobreviverem ao restart.
func chaveMestraSandbox(amb Ambiente, db string) (chave, arquivo string, err error) {
	if v := amb.Getenv("VINTEM_MASTER_KEY"); v != "" {
		return v, "", nil
	}
	arquivo = strings.TrimSuffix(db, ".db") + ".key"
	b, err := os.ReadFile(arquivo) //nolint:gosec // G304: caminho derivado de VINTEM_DB, escolhido pelo operador
	switch {
	case err == nil:
		return string(trimFinal(b)), arquivo, nil
	case !errors.Is(err, fs.ErrNotExist):
		return "", "", fmt.Errorf("lendo a chave do sandbox: %w", err)
	}
	chave = vault.NovaChave()
	if err := os.WriteFile(arquivo, []byte(chave+"\n"), 0o600); err != nil {
		return "", "", fmt.Errorf("gravando a chave do sandbox: %w", err)
	}
	return chave, arquivo, nil
}

// prepararSandbox cria a conta sandbox na primeira vez e uma API key nova com
// todos os escopos a cada subida. As keys anteriores continuam valendo.
func prepararSandbox(ctx context.Context, amb Ambiente, db string) (string, error) {
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return "", err
	}
	defer fechar()
	lista, err := contas.Listar(ctx)
	if err != nil {
		return "", err
	}
	existe := false
	for _, c := range lista {
		// Uma API key com todos os escopos num banco com conta real seria um
		// acidente esperando para acontecer.
		if c.PSP != "sandbox" {
			return "", fmt.Errorf("%s tem a conta %s do PSP %s: o modo sandbox só roda num banco sem contas reais (use outro VINTEM_DB)", db, c.ID, c.PSP)
		}
		existe = existe || c.ID == contaSandbox
	}
	if !existe {
		c := domain.Conta{ID: contaSandbox, Nome: "Sandbox", PSP: "sandbox", ChavePix: chaveSandbox, Ambiente: domain.Homologacao}
		if err := contas.Criar(ctx, c, domain.Credenciais{}); err != nil && !errors.Is(err, ports.ErrJaExiste) {
			return "", err
		}
	}
	_, token, err := app.APIKeys{Store: contas.Store, Clock: amb.Clock}.Criar(ctx, "sandbox", domain.TodosEscopos, "")
	return token, err
}

// urlLocal transforma o endereço de escuta numa URL para colar no terminal.
func urlLocal(addr string) string {
	host, porta, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, porta)
}
