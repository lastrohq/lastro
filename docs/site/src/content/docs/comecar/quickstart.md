---
title: Do zero ao Pix pago
description: Suba o Vintém em modo sandbox e veja uma cobrança Pix ser paga em segundos, sem conta no banco.
---

O sandbox é um banco de mentira com estado: cria a cobrança, gera o BR Code e
"paga" cinco segundos depois. Nada para configurar.

## 1. Subir

Com Go 1.27 ou mais novo:

```bash
go run github.com/vintemhq/vintem/cmd/vintem@latest sandbox
```

Ou com Docker:

```bash
docker run --rm -p 8080:8080 ghcr.io/vintemhq/vintem sandbox
```

O terminal mostra a API, o endereço do painel e uma **API key** de teste. Copie
o `export VINTEM_TOKEN=...` que ele imprime.

## 2. Cobrar

```bash
export VINTEM_TOKEN=vtm_...   # o que o terminal mostrou
curl -s -X POST localhost:8080/cob \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: pedido-42" \
  -H 'Content-Type: application/json' \
  -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@vintem.dev"}'
```

A resposta traz o `txid`, o status `ATIVA` e o `pixCopiaECola`, que é o que o
pagador cola no app do banco.

## 3. Ver ser paga

```bash
curl -s localhost:8080/cob/SEU_TXID -H "Authorization: Bearer $VINTEM_TOKEN"
```

Cinco segundos depois de criada, o status é `CONCLUIDA`. Abra também
`http://localhost:8080/painel/` e entre com a mesma API key.

## Próximos passos

- [Valores mágicos do sandbox](/vintem/gerado/sandbox/): erro do banco, cobrança que expira, pagamento divergente, Pix duplicado.
- [Receber pela sua conta Efí](/vintem/gerado/efi/).
- Integrar pela [SDK TypeScript](/vintem/integrar/sdk/) ou pelo [seu agente de IA](/vintem/integrar/mcp/).
- A [referência da API](/vintem/api/), gerada do `openapi.yaml`.
