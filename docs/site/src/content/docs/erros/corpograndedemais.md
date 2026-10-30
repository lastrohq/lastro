---
title: CorpoGrandeDemais
description: "O corpo de uma requisição com `Idempotency-Key` passa de 1 MiB."
---

**HTTP 413.** O corpo de uma requisição com `Idempotency-Key` passa de 1 MiB.

## Quando acontece

O limite de 1 MiB vale para todas as rotas. Esta resposta sai quando a requisição traz `Idempotency-Key`. Sem o header, o mesmo corpo é recusado com [`RequisicaoInvalida`](/erros/requisicaoinvalida/) e o `detail` `can't decode JSON body: http: request body too large`.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/CorpoGrandeDemais",
  "title": "O corpo passa de 1 MiB",
  "status": 413
}
```

## O que fazer

Reduza o corpo. Nenhuma rota da API precisa de tanto: confira se a aplicação não está mandando o campo errado.

[Todos os tipos de erro](/referencia/erros/)
