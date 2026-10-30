---
title: AcessoNegado
description: "A API key é válida, mas não tem o escopo que a rota exige."
---

**HTTP 403.** A API key é válida, mas não tem o escopo que a rota exige.

## Quando acontece

Cada rota exige um escopo, e a key só faz o que os escopos dela liberam. O `detail` diz qual faltou.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/AcessoNegado",
  "title": "A API key não tem o escopo necessário",
  "status": 403,
  "detail": "api key sem o escopo necessário: precisa de cob:write"
}
```

## O que fazer

Crie uma key com o escopo que aparece no `detail` e use-a nessa chamada:

```bash
vintem apikey criar --nome minha-app --escopo cob:read --escopo cob:write
```

[Todos os tipos de erro](/referencia/erros/)
