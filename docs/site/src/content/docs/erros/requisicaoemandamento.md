---
title: RequisicaoEmAndamento
description: "A primeira requisição com essa `Idempotency-Key` ainda não terminou."
---

**HTTP 409.** A primeira requisição com essa `Idempotency-Key` ainda não terminou.

## Quando acontece

Duas requisições com a mesma chave chegaram juntas, e a segunda recebe este erro com o header `Retry-After: 1`. Uma reserva sem resposta por mais de 2 minutos é tratada como abandonada e liberada (RN-11).

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/RequisicaoEmAndamento",
  "title": "A primeira requisição com essa Idempotency-Key ainda não terminou",
  "status": 409,
  "detail": "já existe uma requisição em andamento com essa Idempotency-Key"
}
```

## O que fazer

Espere o tempo do `Retry-After` e repita a mesma requisição, com a mesma chave. Quando a primeira terminar, você recebe a resposta dela.

[Todos os tipos de erro](/referencia/erros/)
