---
title: WebhookInvalido
description: "O corpo de um webhook não está no formato esperado."
---

**HTTP 400.** O corpo de um webhook não está no formato esperado.

## Quando acontece

O erro é na rota que o banco chama, não na API que a sua aplicação usa: o token da URL está certo, mas o corpo não pôde ser lido, como um JSON sem a lista `pix` ou um Pix com valor ou horário fora do formato.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/WebhookInvalido",
  "title": "O corpo do webhook não está no formato esperado",
  "status": 400,
  "detail": "pix recebido inválido: lista pix vazia"
}
```

## O que fazer

Se foi um teste seu, compare o corpo com o da spec do BCB. Se foi o banco, [abra uma issue](https://github.com/vintemhq/vintem/issues) com o `detail`, sem dados de pagadores.

[Todos os tipos de erro](/referencia/erros/)
