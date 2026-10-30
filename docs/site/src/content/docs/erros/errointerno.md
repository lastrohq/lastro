---
title: ErroInterno
description: "Algo inesperado falhou dentro do Vintém."
---

**HTTP 500.** Algo inesperado falhou dentro do Vintém.

## Quando acontece

A resposta não traz `detail` de propósito, para não vazar nada do servidor. A causa fica no log da instância, na linha `erro interno`.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/ErroInterno",
  "title": "Erro interno",
  "status": 500
}
```

## O que fazer

Se você opera a instância, procure `erro interno` no log. Com a `Idempotency-Key`, pode repetir a chamada sem criar nada em dobro: resposta 5xx não é guardada (RN-11). Se o erro continuar, [abra uma issue](https://github.com/vintemhq/vintem/issues) com o log, sem credenciais.

[Todos os tipos de erro](/referencia/erros/)
