---
title: Cobranças
description: Cobrança imediata (cob) e com vencimento (cobv), do txid ao BR Code, e os estados de cada uma.
---

Há dois tipos de cobrança avulsa. As duas usam a mesma máquina de estados.

| | `cob` (imediata) | `cobv` (com vencimento) |
|---|---|---|
| **Para quê** | Pagar agora: checkout, balcão, pedido | Fatura ou mensalidade avulsa |
| **Validade** | `expiracao` em segundos; padrão de 86400 (24 h) | `dataDeVencimento` mais `validadeAposVencimento` (padrão de 30 dias) |
| **Devedor** | Opcional | Obrigatório: CPF ou CNPJ, e nome |
| **Multa, juros, abatimento, desconto** | Não | Sim. O Vintém valida as combinações, e o cálculo final é do banco |
| **Criar** | `POST /cob` ou `PUT /cob/{txid}` | `PUT /cobv/{txid}` |

## Criar

```bash
curl -X POST http://localhost:8080/cob \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: pedido-42" \
  -H "Content-Type: application/json" \
  -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"sandbox@vintem.dev"}'
```

```json
{
  "calendario": { "criacao": "2026-10-30T13:14:47.601085756Z", "expiracao": 3600 },
  "chave": "sandbox@vintem.dev",
  "expirada": false,
  "location": "sandbox.vintem.dev/qr/v2/sandbox/1",
  "pixCopiaECola": "00020126560014br.gov.bcb.pix2534sandbox.vintem.dev/qr/v2/sandbox/1520400005303986540549.905802BR5907Sandbox6007SANDBOX62070503***6304B618",
  "revisao": 0,
  "status": "ATIVA",
  "txid": "5DMBgQBecrLz6BtZeChvg0n9aCxbX8AL",
  "valor": { "original": "49.90" }
}
```

Uma cobrança com vencimento:

```bash
curl -X PUT http://localhost:8080/cobv/cobvcobvcobvcobvcobvcobvcobv01 \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Content-Type: application/json" \
  -d '{"calendario":{"dataDeVencimento":"2030-12-20"},"devedor":{"cpf":"12345678909","nome":"Fulano de Tal"},"valor":{"original":"120.00","multa":{"modalidade":2,"valorPerc":"2.00"}},"chave":"sandbox@vintem.dev"}'
```

`dataDeVencimento` é uma data civil no fuso de Brasília e não pode estar no passado.

## Valores

O valor é uma string com duas casas decimais e ponto: `"49.90"`. Não vale sinal,
vírgula, separador de milhar ou outro número de casas. O máximo é `9999999999.99`.

## txid

O txid identifica a cobrança e tem de 26 a 35 caracteres `[a-zA-Z0-9]`. Se você não
informar, o Vintém gera um com 32 caracteres. Ele é único na instância inteira e
entre os tipos: o txid de uma `cob` não serve para uma `cobv` nem para uma `cobr`.
Repetir um txid responde [`JaExiste`](/erros/jaexiste/).

## BR Code

O `pixCopiaECola` é o BR Code: o texto que vira QR Code ou é colado no app do
pagador. Ele aponta para a location do banco da conta; o Vintém nunca usa uma
location própria.

## Estados

```
ATIVA ──▶ CONCLUIDA
  │
  ├──▶ REMOVIDA_PELO_USUARIO_RECEBEDOR
  └──▶ REMOVIDA_PELO_PSP
```

Os três estados finais são definitivos, e uma transição fora do desenho responde
[`CobOperacaoInvalida`](/erros/coboperacaoinvalida/).

`CONCLUIDA` só vem de evidência de pagamento, pelo webhook do banco ou pela
conciliação. Nenhuma chamada da sua aplicação conclui uma cobrança.

### Expirada não é um estado

Na spec do BCB, uma cobrança expirada continua `ATIVA`. Para você não precisar
calcular, a `cob` traz o campo `expirada`, que é uma extensão do Vintém.

## Consultar

```bash
curl http://localhost:8080/cob/$TXID -H "Authorization: Bearer $VINTEM_TOKEN"
curl "http://localhost:8080/cob?inicio=2026-01-01T00:00:00Z&fim=2027-01-01T00:00:00Z" \
  -H "Authorization: Bearer $VINTEM_TOKEN"
```

Se a cobrança ainda está `ATIVA`, a consulta também pergunta ao banco. Assim, um
pagamento cujo aviso ainda não chegou já aparece.

## Revisar

`PATCH` altera a cobrança enquanto ela está `ATIVA`, e cada revisão aumenta o campo
`revisao`, que começa em 0.

```bash
curl -X PATCH http://localhost:8080/cob/$TXID \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Content-Type: application/json" \
  -d '{"valor":{"original":"59.90"},"solicitacaoPagador":"Pedido 42"}'
```

Na `cobv`, cada bloco enviado (`multa`, `juros`, `abatimento`, `desconto`) substitui
o anterior inteiro, e o resultado é validado de novo.

## Remover

Remover é um `PATCH` com o status:

```bash
curl -X PATCH http://localhost:8080/cob/$TXID \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Content-Type: application/json" \
  -d '{"status":"REMOVIDA_PELO_USUARIO_RECEBEDOR"}'
```

## Quando o pagamento difere do cobrado

O banco é a fonte da verdade. Um Pix com valor diferente ainda conclui a cobrança, e
o evento `pix.recebido` sai com `divergente: true`. No sandbox, uma cobrança de
`1.01` simula esse caso.

Veja também os [eventos das cobranças](/conceitos/webhooks-de-saida/#catálogo-de-eventos)
e as regras [RN-10 a RN-15](/gerado/regras-de-negocio/).
