---
title: SDK TypeScript
description: "@lastrohq/sdk: cliente tipado do Lastro, gerado do OpenAPI, com verificação de webhook."
---

`@lastrohq/sdk` é gerado do `openapi.yaml`, não tem dependências e roda em Node
20+, Deno, Bun e navegadores.

```bash
npm install @lastrohq/sdk
```

## Criar uma cobrança

```ts
import { Lastro } from "@lastrohq/sdk";

const lastro = new Lastro({ url: "http://localhost:8080", token: process.env.LASTRO_TOKEN! });
const cob = await lastro.cob.criar({
  calendario: { expiracao: 3600 },
  valor: { original: "49.90" },
  chave: "sandbox@lastro.dev",
});
console.log(cob.txid, cob.status, cob.pixCopiaECola);
```

`lastro.cob`, `lastro.cobv` e `lastro.devolucao` cobrem criar, consultar,
listar, revisar e cancelar. Erro da API vira `LastroError`, com `status` e o
`problema` no formato da spec do BCB.

## Verificar um webhook

O Lastro assina os eventos no padrão [Standard Webhooks](https://www.standardwebhooks.com/).
Confira com o corpo **cru**, do jeito que chegou:

```ts
import { verificarWebhook } from "@lastrohq/sdk";

const evento = await verificarWebhook(process.env.LASTRO_WEBHOOK_SECRET!, req.headers, corpoCru);
```

Assinatura errada, corpo alterado ou horário fora da janela de 5 minutos lançam
`AssinaturaInvalida`. O código está em
[`sdk/typescript`](https://github.com/lastrohq/lastro/tree/develop/sdk/typescript).
