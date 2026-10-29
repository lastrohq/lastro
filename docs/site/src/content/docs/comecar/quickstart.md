---
title: Do zero ao Pix pago
description: Suba o Lastro em modo sandbox e veja uma cobrança Pix ser paga em segundos, sem conta no banco.
---

O sandbox é um banco de mentira com estado: cria a cobrança, gera o BR Code e
"paga" cinco segundos depois. Nada para configurar.

## 1. Subir

Com Go 1.27 ou mais novo:

```bash
go run github.com/lastrohq/lastro/cmd/lastro@latest sandbox
```

Ou com Docker:

```bash
docker run --rm -p 8080:8080 ghcr.io/lastrohq/lastro sandbox
```

O terminal mostra a API, o endereço do painel e uma **API key** de teste. Copie
o `export LASTRO_TOKEN=...` que ele imprime.

## 2. Cobrar

```bash
export LASTRO_TOKEN=lst_...   # o que o terminal mostrou
curl -s -X POST localhost:8080/cob \
  -H "Authorization: Bearer $LASTRO_TOKEN" -H "Idempotency-Key: pedido-42" \
  -H 'Content-Type: application/json' \
  -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@lastro.dev"}'
```

A resposta traz o `txid`, o status `ATIVA` e o `pixCopiaECola`, que é o que o
pagador cola no app do banco.

## 3. Ver ser paga

```bash
curl -s localhost:8080/cob/SEU_TXID -H "Authorization: Bearer $LASTRO_TOKEN"
```

Cinco segundos depois de criada, o status é `CONCLUIDA`. Abra também
`http://localhost:8080/painel/` e entre com a mesma API key.

## Próximos passos

- [Valores mágicos do sandbox](/lastro/gerado/sandbox/): erro do banco, cobrança que expira, pagamento divergente, Pix duplicado.
- [Receber pela sua conta Efí](/lastro/gerado/efi/).
- Integrar pela [SDK TypeScript](/lastro/integrar/sdk/) ou pelo [seu agente de IA](/lastro/integrar/mcp/).
- A [referência da API](/lastro/api/), gerada do `openapi.yaml`.
