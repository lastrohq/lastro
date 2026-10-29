---
title: Use com seu agente (MCP)
description: lastro mcp dá a agentes de IA ferramentas para cobrar, consultar e devolver Pix, com os escopos da API key.
---

`lastro mcp` é um servidor MCP por stdio. Ele chama a API do Lastro com a API
key do agente, então os escopos valem: uma key só de leitura dá um agente que
só consulta ([ADR-008](/lastro/gerado/adr/008-mcp-como-cliente-da-api/)).

## No Claude Code

```bash
lastro sandbox   # copie a key lst_... que ele imprime
claude mcp add lastro -e LASTRO_TOKEN=lst_... -e LASTRO_CHAVE_PIX=sandbox@lastro.dev -- lastro mcp
```

Depois peça: *"cobre R$ 49,90 do pedido 42 e me mostre o Pix copia e cola"*.

## Em qualquer cliente MCP

Rode `lastro mcp` por stdio com estas variáveis:

| Variável | Para quê |
|---|---|
| `LASTRO_URL` | Onde está a API (padrão `http://localhost:8080`) |
| `LASTRO_TOKEN` | A API key do agente |
| `LASTRO_CHAVE_PIX` | Chave usada quando o agente não informa outra |

## Ferramentas

| Ferramenta | Escopo |
|---|---|
| `criar_cobranca` | `cob:write` |
| `consultar_cobranca` | `cob:read` |
| `listar_cobrancas` | `cob:read` |
| `cancelar_cobranca` | `cob:write` |
| `devolver_pix` | `devolucao:write` |
| `consultar_devolucao` | `pix:read` |
