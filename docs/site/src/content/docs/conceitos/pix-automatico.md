---
title: Pix Automático
description: Recorrência, solicitação de confirmação e cobrança recorrente, com as jornadas do BCB e os estados de cada uma.
---

No Pix Automático, o pagador autoriza uma vez e as cobranças seguintes são pagas
sozinhas, como um débito automático. São três peças:

| Peça | O que é | Rota |
|---|---|---|
| **Recorrência** | O acordo: periodicidade, valor, vigência e retentativa | `/rec` |
| **Solicitação** | O pedido de autorização, enviado ao app do pagador | `/solicrec` |
| **Cobrança recorrente** | A cobrança de um ciclo do acordo | `/cobr` |

:::caution[Hoje, só no sandbox]
Nenhum adapter de banco real declara `rec` e `cobr`, então pedir Pix Automático numa
conta real responde [`NaoSuportado`](/erros/naosuportado/). Efí e Inter, que documentam
essa API, só liberam o recurso para conta PJ. Tudo nesta página pode ser experimentado
no [sandbox](/gerado/sandbox/).
:::

## Antes de começar

A recorrência identifica o recebedor pela API key, então ela precisa ser **restrita a
uma conta**. Uma key sem conta, como a do `vintem sandbox`, responde
[`RequisicaoInvalida`](/erros/requisicaoinvalida/).

```bash
vintem apikey criar --nome loja --conta sandbox \
  --escopo rec:read --escopo rec:write --escopo cobr:read --escopo cobr:write
```

Os exemplos usam datas relativas a hoje, porque a cobrança tem uma janela de
antecedência. No macOS, troque por `date -v+3d +%F`, e assim por diante.

```bash
INICIO=$(date -d '+3 days' +%F)
VENCIMENTO=$(date -d '+6 days' +%F)
NOVA_DATA=$(date -d '+8 days' +%F)
EXPIRA=$(date -u -d '+10 days' +%FT%TZ)
```

## 1. Criar a recorrência

```bash
curl -X POST http://localhost:8080/rec \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: contrato-63100862" \
  -H "Content-Type: application/json" \
  -d '{"vinculo":{"contrato":"63100862","objeto":"Academia","devedor":{"cpf":"12345678909","nome":"Fulano de Tal"}},"calendario":{"dataInicial":"'$INICIO'","periodicidade":"MENSAL"},"valor":{"valorRec":"89.90"},"politicaRetentativa":"PERMITE_3R_7D"}'
```

- **Periodicidade:** `SEMANAL`, `MENSAL`, `TRIMESTRAL`, `SEMESTRAL` ou `ANUAL`.
- **Valor:** fixo (`valorRec`) ou variável. No variável, o pagador define um máximo na
  autorização, e você pode exigir um piso para esse máximo (`valorMinimoRecebedor`).
  Fixo e piso são excludentes.
- **Retentativa:** `NAO_PERMITE` ou `PERMITE_3R_7D`. Só o recebedor define.

## 2. Pedir a autorização

A autorização é dada por uma de quatro **jornadas** do BCB:

| Jornada | Como o pagador autoriza |
|---|---|
| 1 | Notificação no app dele, sem QR Code (`/solicrec`) |
| 2 | QR Code só com a recorrência |
| 3 | QR Code com um primeiro pagamento imediato e a recorrência |
| 4 | QR Code de fatura com a recorrência |

A API do Vintém tem rota para a jornada 1. As outras dependem de QR Code de recorrência,
que ainda não está na API.

```bash
curl -X POST http://localhost:8080/solicrec \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: solic-63100862" \
  -H "Content-Type: application/json" \
  -d '{"idRec":"'$ID_REC'","calendario":{"dataExpiracaoSolicitacao":"'$EXPIRA'"},"destinatario":{"conta":"123456","ispbParticipante":"18236120","agencia":"0001"}}'
```

A solicitação fica pendente por no máximo 30 dias e depois expira. Enquanto o pagador
não responde, você pode consultar e desistir. A recorrência continua `CRIADA` e pode
pedir outra confirmação:

```bash
curl http://localhost:8080/solicrec/$ID_SOLIC -H "Authorization: Bearer $VINTEM_TOKEN"
curl -X PATCH http://localhost:8080/solicrec/$ID_SOLIC \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Content-Type: application/json" \
  -d '{"status":"CANCELADA"}'
```

O cancelamento publica de novo `recorrencia.solicitacao`, com o status `CANCELADA`.

## 3. Saber a resposta

A resposta chega pelo webhook de recorrência, que `vintem conta webhook` registra com
o mesmo token do aviso de Pix. Se o aviso se perder, a
[conciliação](/conceitos/recebimentos-e-conciliacao/) encontra a mudança. Nos dois
casos, sai o evento `recorrencia.atualizada`.

## 4. Cobrar um ciclo

A cobrança só nasce de uma recorrência `APROVADA`.

```bash
curl -X PUT http://localhost:8080/cobr/cobrcobrcobrcobrcobrcobrcobr01 \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Content-Type: application/json" \
  -d '{"idRec":"'$ID_REC'","calendario":{"dataDeVencimento":"'$VENCIMENTO'"},"valor":{"original":"89.90"},"infoAdicional":"Mensalidade"}'
```

- O vencimento fica dentro da vigência da recorrência. Se ela tem valor fixo, a
  cobrança tem esse valor.
- **Antecedência:** a instrução chega ao banco do pagador de 2 a 10 dias corridos antes
  da liquidação. O Vintém recusa fora dessa janela.
- **Uma cobrança por ciclo.** Enquanto a do ciclo está `CRIADA`, `ATIVA` ou `CONCLUIDA`,
  outra é recusada. Depois de cancelada, rejeitada ou expirada, você pode criar outra.

## Quando o pagamento falha

Se a recorrência é `PERMITE_3R_7D`, você pode pedir uma nova tentativa:

```bash
curl -X POST http://localhost:8080/cobr/$TXID/retentativa/$NOVA_DATA \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: retry-1"
```

- No máximo 3 datas diferentes, em até 7 dias corridos da data original.
- Sem passar do início do ciclo seguinte nem do fim da recorrência.
- Com o mesmo valor.
- Pedida até 23h59 do dia anterior à nova data, no fuso de Brasília.

A cobrança continua `ATIVA` durante as retentativas. O andamento fica nas `tentativas`,
identificadas pela data e pelo tipo: `SOLICITADA → AGENDADA → PAGA`, ou `REJEITADA`,
`CANCELADA` e `EXPIRADA`.

## Estados

Os diagramas seguem o *Manual de Padrões para Iniciação do Pix* 2.10.0 do BCB.

### Recorrência

```
CRIADA ──▶ APROVADA ──▶ CANCELADA
   │          └───────▶ EXPIRADA
   ├──▶ REJEITADA
   ├──▶ EXPIRADA
   └──▶ CANCELADA
```

`REJEITADA` só acontece na jornada 1. O motivo fica gravado.

### Solicitação

```
CRIADA ──▶ ENVIADA ──▶ RECEBIDA ──▶ ACEITA
   │          ├───────────┼────────▶ REJEITADA
   ├──────────┼───────────┼────────▶ EXPIRADA
   └──────────┴───────────┴────────▶ CANCELADA
```

`ACEITA` leva a recorrência a `APROVADA`, e `REJEITADA` a `REJEITADA`.

### Cobrança recorrente

```
CRIADA ──▶ ATIVA ──▶ CONCLUIDA
   │         ├─────▶ EXPIRADA
   │         ├─────▶ REJEITADA
   └─────────┴─────▶ CANCELADA
```

`CRIADA` é a cobrança registrada no banco do recebedor; `ATIVA`, a enviada ao banco do
pagador. O recebedor só cancela, com um `PATCH` para `CANCELADA`.

## Consultar e acompanhar

`GET /cobr/{txid}` pergunta ao banco enquanto a cobrança está aberta e devolve a versão
gravada se ele falhar. Os avisos chegam pelo webhook de cobrança recorrente, e a
conciliação confere as abertas.

Os eventos estão no [catálogo](/conceitos/webhooks-de-saida/#catálogo-de-eventos), e
as regras completas são a [RN-40 a RN-43](/gerado/regras-de-negocio/).

## Testar no sandbox

O valor fixo da recorrência escolhe o cenário:

| Valor fixo | O que acontece |
|---|---|
| `0.44` | O banco do pagador rejeita a cobrança: `REJEITADA` |
| `0.55` | A tentativa do vencimento falha (`AM04`), a cobrança segue `ATIVA`, e a retentativa é paga |
| `0.66` | O sandbox recusa criar |
| `0.77` | O pagador rejeita a recorrência (`AP13`) |
| `0.88` | O pagador não responde: a solicitação expira |
| Outro | O pagador aprova, e a cobrança é paga no vencimento: `CONCLUIDA` |
