# ADR-005 · Webhooks de saída no padrão Standard Webhooks, com outbox

**Status:** Aceita · **Data:** 28/10/2026

## Contexto

Webhook de Pix é o ponto mais citado como problemático por quem integra: cada
banco manda num formato, alguns exigem mTLS no recebedor, e aviso perdido vira
pedido não liberado.

## Decisão

- Saída no padrão [Standard Webhooks](https://www.standardwebhooks.com/) (`webhook-id`, `webhook-timestamp`, `webhook-signature` HMAC-SHA256), com payload normalizado independente do banco.
- Evento gravado numa tabela **outbox** na mesma transação da mudança de estado; workers entregam com backoff exponencial + jitter por até 24h, depois DLQ.
- Entrada deduplicada por `endToEndId`, e um job de conciliação (`GET /pix`) cobre o aviso que o banco não mandou.

## Consequências

- ➕ Clientes usam bibliotecas prontas de verificação de assinatura em qualquer linguagem.
- ➕ Sem Redis/fila externa; nenhuma mudança de estado fica sem evento.
- ➖ Entrega *at-least-once*: o cliente precisa deduplicar pelo `webhook-id`.
- ➖ Outbox no SQLite exige cuidado com contenção de escrita (workers em lote).
