# @lastrohq/sdk

SDK TypeScript do [Lastro](https://github.com/lastrohq/lastro), o gateway Pix
self-hosted. Os tipos são gerados do `api/openapi.yaml`, então acompanham a API.
Sem dependências; roda em Node 20+, Deno, Bun e navegadores.

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

Para testar sem banco, suba o `lastro sandbox` e use a API key que ele imprime.

| | |
|---|---|
| `lastro.cob` | `criar`, `criarComTxid`, `consultar`, `listar`, `revisar`, `cancelar` |
| `lastro.cobv` | `criar`, `consultar`, `listar`, `revisar` |
| `lastro.devolucao` | `solicitar`, `consultar` |

Erro da API vira `LastroError`, com `status` e o `problema` no formato da spec
do BCB. `cob.criar` manda uma `Idempotency-Key` nova a cada chamada; passe a sua
(`{ idempotencyKey }`) para tentar de novo sem criar duas cobranças.

## Verificar um webhook

O Lastro assina os eventos no padrão [Standard Webhooks](https://www.standardwebhooks.com/).
Confira com o corpo **cru**, como chegou:

```ts
import { verificarWebhook, AssinaturaInvalida } from "@lastrohq/sdk";

app.post("/webhooks/lastro", express.text({ type: "*/*" }), async (req, res) => {
  try {
    const evento = await verificarWebhook(process.env.LASTRO_WEBHOOK_SECRET!, req.headers, req.body);
    // evento.type, evento.data
    res.sendStatus(204);
  } catch (e) {
    res.sendStatus(e instanceof AssinaturaInvalida ? 401 : 500);
  }
});
```

## Desenvolvimento

```bash
npm ci
npm run gerar   # regenera src/schema.ts do api/openapi.yaml
LASTRO_URL=http://localhost:8080 LASTRO_TOKEN=lst_... npm test
```

Sem `LASTRO_URL`/`LASTRO_TOKEN`, os testes contra o sandbox são pulados. A
publicação no npm sai do CI a partir de uma tag `sdk-vX.Y.Z` igual à versão do
`package.json`.
