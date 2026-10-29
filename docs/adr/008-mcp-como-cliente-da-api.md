# ADR-008 · O servidor MCP é um cliente da API HTTP, não um atalho para o banco

**Status:** Aceita · **Data:** 29/10/2026

## Contexto

A #38 pede um `lastro mcp` com ferramentas de cobrança para agentes de IA, com
os escopos da API key valendo nas ferramentas de escrita. A ARQUITETURA previa o
pacote `internal/adapters/mcp` chamando os casos de uso direto, como a CLI faz.

Havia dois caminhos:
- **Chamar os casos de uso no mesmo processo**, abrindo o banco e o vault: exige
  `LASTRO_MASTER_KEY` no cliente do agente, só funciona na máquina onde o banco
  está e precisa reimplementar a checagem de escopo, que hoje mora no `httpapi`.
- **Falar com a API HTTP** do Lastro que já está no ar, com a API key do agente.

## Decisão

- `lastro mcp` roda por stdio e chama a API em `LASTRO_URL` (padrão
  `http://localhost:8080`) com `Authorization: Bearer $LASTRO_TOKEN`.
- Cada ferramenta é uma rota da API. Escopo, conta da key, idempotência e
  validação valem porque é a mesma API que qualquer cliente usa; o 403 chega ao
  agente como erro da ferramenta, com o motivo.
- `LASTRO_CHAVE_PIX` é a chave usada quando o agente não informa outra, porque
  `chave` é obrigatória na cob e o agente não tem como adivinhá-la.
- SDK oficial: `github.com/modelcontextprotocol/go-sdk`.

## Consequências

- **Melhor:** o agente não vê a chave mestra nem o banco; o mesmo `lastro mcp`
  serve um Lastro remoto ou o `lastro sandbox` local; uma key só de leitura dá um
  agente que só consulta.
- **Pior:** o Lastro precisa estar no ar para o MCP funcionar, e cada ferramenta
  é uma chamada HTTP a mais. Ferramentas novas dependem de existir a rota.
