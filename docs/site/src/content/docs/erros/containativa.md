---
title: ContaInativa
description: "A chave Pix informada pertence a uma conta desativada."
---

**HTTP 400.** A chave Pix informada pertence a uma conta desativada.

## Quando acontece

Conta desativada não aceita cobranças novas (RN-03).

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/ContaInativa",
  "title": "A conta da chave informada está desativada",
  "status": 400,
  "detail": "conta sandbox: conta inativa"
}
```

## O que fazer

Use a chave de uma conta ativa. `GET /vintem/v1/contas` mostra o campo `ativa` de cada conta.

[Todos os tipos de erro](/referencia/erros/)
