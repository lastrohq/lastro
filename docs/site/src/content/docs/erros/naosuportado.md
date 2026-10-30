---
title: NaoSuportado
description: "O banco da conta não oferece essa operação."
---

**HTTP 501.** O banco da conta não oferece essa operação.

## Quando acontece

Nem todo banco tem todas as capacidades. Hoje isso acontece ao pedir Pix Automático (`rec` e `cobr`) numa conta de banco real, porque só o sandbox oferece essas capacidades. A instância também responde este erro para devoluções, se tiver subido sem elas (`as devoluções não estão configuradas nesta instância`).

## Exemplo

```json
{
  "type": "https://vintem.dev/erros/NaoSuportado",
  "title": "O PSP da conta não oferece essa operação",
  "status": 501,
  "detail": "capacidade não suportada pelo PSP: rec"
}
```

## O que fazer

Use uma conta de um banco que tenha a capacidade do `detail`. Para experimentar o Pix Automático, use o [sandbox](/gerado/sandbox/).

[Todos os tipos de erro](/referencia/erros/)
