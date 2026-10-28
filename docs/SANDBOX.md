# 🧪 Sandbox

O adapter `sandbox` é um PSP de mentira: implementa as capacidades do Lastro
sem banco nenhum, para você testar o fluxo inteiro **sem conta PJ** (RN-52).

```bash
export LASTRO_MASTER_KEY=$(lastro chave gerar)
lastro conta criar --id teste --nome "Minha Loja" --psp sandbox --chave teste@example.com
```

## Do zero a um Pix pago

```bash
export LASTRO_MASTER_KEY=$(lastro chave gerar)
lastro conta criar --id loja --nome "Minha Loja" --psp sandbox --chave loja@example.com
lastro serve &

curl -X PUT localhost:8080/cob/pedido00000000000000000000001 \
  -H 'Content-Type: application/json' \
  -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"loja@example.com"}'
# → 201, status ATIVA, com o pixCopiaECola

sleep 5
curl localhost:8080/cob/pedido00000000000000000000001
# → status CONCLUIDA, revisao 1
```

A consulta pergunta ao PSP quando a cobrança ainda está `ATIVA`; é assim que
ela vira `CONCLUIDA` antes de os webhooks existirem (F4).

## Como ele se comporta

Toda cobrança é **paga 5 segundos depois de criada**, desde que não tenha
expirado antes. O pagamento aparece na consulta da cobrança (`CONCLUIDA`) e em
`GET /pix`.

## Valores mágicos

O valor da cobrança escolhe o cenário, como os cartões de teste dos gateways:

| Valor | O que acontece | Para testar |
|---|---|---|
| `0.66` | O PSP recusa a criação | Erro do banco na criação |
| `0.99` | Nunca é paga; a cobrança expira | Expiração e cobrança abandonada |
| `1.01` | É paga com **R$ 1,00** | Pagamento divergente do cobrado |
| `2.02` | O mesmo Pix chega **duas vezes** | Deduplicação por `endToEndId` |
| qualquer outro | Paga integralmente em 5 s | Caminho feliz |

## O que ainda não faz

- **Webhook:** a URL é aceita, mas o aviso de pagamento só passa a ser entregue
  com a F4 (#22, #23). Até lá, consulte a cobrança ou `GET /pix`.
- **Estado:** fica em memória. Reiniciar o Lastro esquece as cobranças do sandbox.
- **cobv e Pix Automático:** entram com as fases deles.
