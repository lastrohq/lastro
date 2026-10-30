---
title: CobOperacaoInvalida
description: "A cobrança não aceita a operação no estado em que está, ou mudou durante ela."
---

**HTTP 400 ou 409.** A cobrança não aceita a operação no estado em que está, ou mudou durante ela.

## Quando acontece

- **400:** a cobrança está num estado que não permite a operação, como revisar uma cobrança já `CONCLUIDA`.
- **409:** outra requisição alterou a cobrança ao mesmo tempo.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/CobOperacaoInvalida",
  "title": "A cobrança não está num estado que permita essa operação",
  "status": 400,
  "detail": "cobrança 7IfvmUGZsXlgfw3cv4nYAYsGOes3aiYw está CONCLUIDA: transição de estado inválida"
}
```

## O que fazer

- **400:** consulte a cobrança com `GET` e confira o `status`. Repetir a mesma chamada não muda o resultado.
- **409:** consulte a cobrança de novo e, se a operação ainda fizer sentido, repita.

[Todos os tipos de erro](/referencia/erros/)
