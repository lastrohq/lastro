---
title: NaoEncontrado
description: "O recurso pedido não existe."
---

**HTTP 404.** O recurso pedido não existe.

## Quando acontece

A cobrança, recorrência, conta ou devolução não existe nesta instância.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/NaoEncontrado",
  "title": "Recurso não encontrado",
  "status": 404,
  "detail": "cobrança naoexiste0000000000000000000001: não encontrado"
}
```

## O que fazer

Confira o identificador e se a API key é da mesma instância que criou o recurso.

[Todos os tipos de erro](/referencia/erros/)
