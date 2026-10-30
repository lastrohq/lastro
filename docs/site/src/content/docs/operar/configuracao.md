---
title: Configuração
description: Todas as variáveis de ambiente do Vintém, com o padrão e o significado de cada uma.
---

O Vintém é configurado por variáveis de ambiente. Tempos usam o formato de duração do
Go: `10s`, `5m`, `2h`.

## Servidor

| Variável | Padrão | O que faz |
|---|---|---|
| `VINTEM_MASTER_KEY` | Obrigatória | Cifra as credenciais dos bancos. Gere com `vintem chave gerar`. Veja [Segurança](/operar/seguranca/) |
| `VINTEM_MASTER_KEY_NOVA` | Nenhum | A chave de destino de `vintem chave trocar`. Só serve para esse comando |
| `VINTEM_DB` | `vintem.db` | Caminho do banco SQLite. O arquivo é criado se não existir |
| `VINTEM_ADDR` | `:8080` | Endereço em que a API escuta |

## Jobs

| Variável | Padrão | O que faz |
|---|---|---|
| `VINTEM_CONCILIACAO_INTERVALO` | `5m` | Frequência da [conciliação](/conceitos/recebimentos-e-conciliacao/). `0` desliga, com aviso no log |
| `VINTEM_CONCILIACAO_JANELA` | `2h` | Até onde cada rodada olha para trás. Zero é recusado. Também vale para `vintem conciliar` |
| `VINTEM_WEBHOOK_INTERVALO` | `2s` | Frequência da entrega de [eventos](/conceitos/webhooks-de-saida/). `0` desliga, e os eventos ficam parados na fila |
| `VINTEM_CERTIFICADO_INTERVALO` | `1h` | Frequência do [aviso de vencimento](/conceitos/contas-e-certificados/#aviso-de-vencimento). `0` desliga |

## Servidor MCP

Usadas pelo [`vintem mcp`](/integrar/mcp/), que fala com a API de uma instância.

| Variável | Padrão | O que faz |
|---|---|---|
| `VINTEM_URL` | Montada a partir de `VINTEM_ADDR`: `http://localhost:8080` | Onde está a API |
| `VINTEM_TOKEN` | Obrigatória | A API key do agente |
| `VINTEM_CHAVE_PIX` | Nenhum | Chave Pix usada quando o agente não informa outra |

## No modo sandbox

O `vintem sandbox` muda três padrões, para funcionar sem configurar nada:

| Variável | Padrão no sandbox |
|---|---|
| `VINTEM_DB` | `vintem-sandbox.db` |
| `VINTEM_MASTER_KEY` | Gerada na primeira subida e guardada em `vintem-sandbox.key`, ao lado do banco |
| `VINTEM_CONCILIACAO_INTERVALO` | `10s` |

Por segurança, ele se recusa a rodar num banco que já tenha uma conta de banco real.
