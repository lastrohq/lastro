---
title: Erros da API
description: O formato dos erros do Vintém e a lista de todos os tipos.
---

Todo erro da API sai em `application/problem+json`, no formato da spec do BCB
(RFC 7807). O campo `type` é o link para a página do erro.

```json
{
  "type": "https://vintem.dev/erros/NaoEncontrado",
  "title": "Recurso não encontrado",
  "status": 404,
  "detail": "cobrança naoexiste0000000000000000000001: não encontrado"
}
```

| Campo | O que traz |
|---|---|
| `type` | O tipo do erro, como link. É o campo para a sua aplicação comparar. |
| `title` | Resumo do erro, para pessoas. Pode mudar entre versões. |
| `status` | O mesmo status HTTP da resposta. |
| `detail` | O caso concreto, para pessoas. Falta quando revelaria algo, como em `NaoAutenticado` e `ErroInterno`. |
| `violacoes` | Em `RequisicaoInvalida` por campo: `propriedade`, `razao` e `valor` de cada problema. |

## Tipos

| Tipo | HTTP | Quando |
|---|---|---|
| [`AcessoNegado`](/erros/acessonegado/) | 403 | A API key é válida, mas não tem o escopo que a rota exige. |
| [`CobOperacaoInvalida`](/erros/coboperacaoinvalida/) | 400 ou 409 | A cobrança não aceita a operação no estado em que está, ou mudou durante ela. |
| [`ContaInativa`](/erros/containativa/) | 400 | A chave Pix informada pertence a uma conta desativada. |
| [`CorpoGrandeDemais`](/erros/corpograndedemais/) | 413 | O corpo de uma requisição com `Idempotency-Key` passa de 1 MiB. |
| [`ErroInterno`](/erros/errointerno/) | 500 | Algo inesperado falhou dentro do Vintém. |
| [`IdempotencyKeyReutilizada`](/erros/idempotencykeyreutilizada/) | 409 | A `Idempotency-Key` já foi usada com outra requisição. |
| [`JaExiste`](/erros/jaexiste/) | 409 | Já existe um recurso com esse identificador. |
| [`NaoAutenticado`](/erros/naoautenticado/) | 401 | A requisição não trouxe uma API key válida. |
| [`NaoEncontrado`](/erros/naoencontrado/) | 404 | O recurso pedido não existe. |
| [`NaoImplementado`](/erros/naoimplementado/) | 501 | A rota existe no contrato, mas ainda não tem implementação. |
| [`NaoSuportado`](/erros/naosuportado/) | 501 | O banco da conta não oferece essa operação. |
| [`PSPIndisponivel`](/erros/pspindisponivel/) | 502 | O banco recusou a operação ou não respondeu. |
| [`RequisicaoEmAndamento`](/erros/requisicaoemandamento/) | 409 | A primeira requisição com essa `Idempotency-Key` ainda não terminou. |
| [`RequisicaoInvalida`](/erros/requisicaoinvalida/) | 400 | A requisição viola o contrato da API ou uma regra de negócio. |
| [`WebhookInvalido`](/erros/webhookinvalido/) | 400 | O corpo de um webhook não está no formato esperado. |
