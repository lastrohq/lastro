---
title: JaExiste
description: "Já existe um recurso com esse identificador."
---

**HTTP 409.** Já existe um recurso com esse identificador.

## Quando acontece

O caso mais comum é um `PUT /cob/{txid}` ou `PUT /cobv/{txid}` com um `txid` que já foi usado.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/JaExiste",
  "title": "Já existe um recurso com esse identificador",
  "status": 409,
  "detail": "cobrança 7IfvmUGZsXlgfw3cv4nYAYsGOes3aiYw: já existe"
}
```

## O que fazer

Consulte o recurso com `GET` para ver se é o que você queria criar. Se for outra operação, use outro identificador.

[Todos os tipos de erro](/referencia/erros/)
