---
title: IdempotencyKeyReutilizada
description: "A `Idempotency-Key` já foi usada com outra requisição."
---

**HTTP 409.** A `Idempotency-Key` já foi usada com outra requisição.

## Quando acontece

A chave pertence à API key que a enviou e vale por 24 h. Repetir a mesma requisição devolve a resposta original; mudar o método, o caminho ou o corpo cai neste erro (RN-11).

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/IdempotencyKeyReutilizada",
  "title": "A Idempotency-Key já foi usada com outra requisição",
  "status": 409,
  "detail": "Idempotency-Key já usada com outra requisição"
}
```

## O que fazer

Gere uma chave nova para cada operação diferente, por exemplo a partir do id do pedido. Se a intenção era repetir a mesma operação, mande o corpo idêntico ao da primeira vez.

[Todos os tipos de erro](/referencia/erros/)
