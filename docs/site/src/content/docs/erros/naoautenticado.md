---
title: NaoAutenticado
description: "A requisição não trouxe uma API key válida."
---

**HTTP 401.** A requisição não trouxe uma API key válida.

## Quando acontece

O header `Authorization: Bearer vtm_...` faltou, ou a key está malformada, revogada ou não existe. A resposta não diz qual dos casos foi e não traz `detail`, para não revelar se a key existe.

As rotas de webhook (`/webhooks/...`) respondem o mesmo erro quando o token da URL está errado ou a conta não existe, embora o `title` fale de API key.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/NaoAutenticado",
  "title": "Envie uma API key válida em Authorization: Bearer vtm_...",
  "status": 401
}
```

## O que fazer

Confira o header e se a key não foi revogada. Se precisar, crie outra com `vintem apikey criar`. No caso do webhook, cadastre a URL de novo com `vintem conta webhook <id> --url-publica URL`.

[Todos os tipos de erro](/referencia/erros/)
