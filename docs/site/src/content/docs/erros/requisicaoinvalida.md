---
title: RequisicaoInvalida
description: "A requisição viola o contrato da API ou uma regra de negócio."
---

**HTTP 400.** A requisição viola o contrato da API ou uma regra de negócio.

## Quando acontece

Campo fora do formato, regra de negócio violada, header obrigatório ausente (como a `Idempotency-Key` no `POST /cob`) ou JSON que não abre. Quando o problema é num campo, a resposta traz `violacoes`, com a propriedade, a razão e o valor recebido.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/RequisicaoInvalida",
  "title": "A requisição viola o schema ou uma regra de negócio",
  "status": 400,
  "detail": "requisição inválida: valor.original: valor inválido: \"abc\" (formato esperado: 10.50)",
  "violacoes": [
    {
      "propriedade": "valor.original",
      "razao": "valor inválido: \"abc\" (formato esperado: 10.50)",
      "valor": "abc"
    }
  ]
}
```

## O que fazer

Corrija cada item de `violacoes`, ou o que o `detail` apontar, e confira a rota na [referência da API](/api/). Repetir sem mudar nada dá o mesmo erro.

[Todos os tipos de erro](/referencia/erros/)
