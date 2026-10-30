---
title: NaoImplementado
description: "A rota existe no contrato, mas ainda não tem implementação."
---

**HTTP 501.** A rota existe no contrato, mas ainda não tem implementação.

## Quando acontece

O tipo está reservado: nenhuma rota responde com ele hoje.

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/NaoImplementado",
  "title": "Ainda não implementado",
  "status": 501,
  "detail": "rota ainda não implementada"
}
```

## O que fazer

Atualize o Vintém para a versão mais nova. Se a resposta continuar, [abra uma issue](https://github.com/vintemhq/vintem/issues) dizendo qual rota chamou.

[Todos os tipos de erro](/referencia/erros/)
