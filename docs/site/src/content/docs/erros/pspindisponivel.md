---
title: PSPIndisponivel
description: "O banco recusou a operação ou não respondeu."
---

**HTTP 502.** O banco recusou a operação ou não respondeu.

## Quando acontece

O Vintém chegou a chamar o banco, e o `detail` traz o que voltou de lá. No sandbox, o valor `0.66` provoca este erro.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/PSPIndisponivel",
  "title": "O PSP recusou ou falhou",
  "status": 502,
  "detail": "o PSP recusou ou falhou: sandbox: o PSP recusou a cobrança (valor mágico 0.66)"
}
```

## O que fazer

Leia o `detail`. Se for uma recusa, corrija o que o banco apontou. Se for falha de rede ou instabilidade, tente de novo com a mesma `Idempotency-Key`: resposta 5xx não é guardada (RN-11).

[Todos os tipos de erro](/referencia/erros/)
