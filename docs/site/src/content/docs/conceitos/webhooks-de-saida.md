---
title: Webhooks de saída
description: Como a sua aplicação recebe os eventos do Vintém, como conferir a assinatura e o catálogo completo de eventos.
---

O Vintém avisa a sua aplicação de cada mudança por um POST assinado, no padrão
[Standard Webhooks](https://www.standardwebhooks.com/). O formato é o mesmo para
qualquer banco.

## Cadastrar um destino

```bash
vintem destino criar --url https://sua-app.com/webhooks/vintem
```

Sem `--conta ID`, o destino recebe os eventos de todas as contas. A URL precisa ser
https; `http` só vale para `localhost`. O segredo `whsec_...` aparece **uma vez**,
na criação.

Evento sem destino cadastrado na hora é descartado: um destino novo não recebe o passado.

## O que chega

```http
POST /webhooks/vintem HTTP/1.1
Content-Type: application/json
Webhook-Id: 270e9d2715d3ab32ee0855e1d4681955
Webhook-Timestamp: 1793366125
Webhook-Signature: v1,yMmJU9nQk...
```

```json
{
  "type": "cobranca.concluida",
  "timestamp": "2026-10-30T13:14:53.457489035Z",
  "data": {
    "tipo": "cob",
    "txid": "5DMBgQBecrLz6BtZeChvg0n9aCxbX8AL",
    "contaId": "sandbox",
    "status": "CONCLUIDA",
    "valor": "49.90",
    "revisao": 1
  }
}
```

## Conferir a assinatura

Confira sempre, e com o corpo **cru**, do jeito que chegou. Reserializar o JSON muda
os bytes e invalida a assinatura.

Com o [SDK](/integrar/sdk/):

```ts
import { verificarWebhook } from "@vintemhq/sdk";

const evento = await verificarWebhook(process.env.VINTEM_WEBHOOK_SECRET!, req.headers, corpoCru);
```

Sem o SDK, em qualquer linguagem: a assinatura é o HMAC-SHA256 de
`id.timestamp.corpo`, em base64, com a chave em base64 depois do prefixo `whsec_`.

```python
import base64, hashlib, hmac, time

def verificar(segredo: str, id: str, timestamp: str, assinatura: str, corpo: bytes) -> bool:
    if abs(time.time() - int(timestamp)) > 300:  # recusa evento reenviado muito depois
        return False
    chave = base64.b64decode(segredo.removeprefix("whsec_"))
    msg = f"{id}.{timestamp}.".encode() + corpo
    esperada = "v1," + base64.b64encode(hmac.new(chave, msg, hashlib.sha256).digest()).decode()
    return hmac.compare_digest(esperada, assinatura)
```

As bibliotecas oficiais do Standard Webhooks também funcionam.

## O que a sua aplicação precisa fazer

- **Responder 2xx rápido.** Qualquer 2xx é sucesso. O timeout é de 10 segundos, e
  redirecionamento não é seguido: conta como falha.
- **Deduplicar pelo `webhook-id`.** A entrega é *at-least-once*, então o mesmo evento
  pode chegar mais de uma vez. O id e o corpo são os mesmos em todas as tentativas.
- **Não depender da ordem.** As entregas correm em paralelo, e uma tentativa nova
  pode chegar depois de um evento mais recente. Use `timestamp` e, onde houver,
  `revisao` para descartar o que é antigo.

## Tentativas e DLQ

Uma entrega que falha é tentada de novo depois de 5 s, 30 s, 2 min, 10 min, 30 min,
1 h, 2 h e depois a cada 4 h, com até 10% de variação, até 24 h depois do evento.
Depois disso, ela vai para a **DLQ**:

```bash
vintem dlq listar
vintem dlq reenviar <evento-id>
```

A tela de webhooks do [painel](/integrar/painel/) mostra o mesmo e tem o botão de
reenvio. Remover um destino manda as entregas pendentes dele para a DLQ.

## Catálogo de eventos

| Evento | Quando dispara | `data` |
|---|---|---|
| `cobranca.criada` | Uma `cob` ou `cobv` foi criada | Cobrança |
| `cobranca.revisada` | Um `PATCH` alterou a cobrança | Cobrança |
| `cobranca.concluida` | O pagamento foi confirmado, por webhook, conciliação ou consulta | Cobrança |
| `cobranca.removida` | A cobrança foi removida pelo recebedor ou pelo banco | Cobrança |
| `pix.recebido` | Um Pix foi registrado pela primeira vez, com ou sem cobrança | Pix |
| `devolucao.solicitada` | O banco aceitou uma devolução | Devolução |
| `devolucao.atualizada` | O banco informou o resultado de uma devolução | Devolução |
| `certificado.expirando` | Faltam 30 dias ou menos para o certificado vencer, ou ele já venceu | Certificado |
| `recorrencia.criada` | Uma recorrência foi criada | Recorrência |
| `recorrencia.atualizada` | O status da recorrência mudou | Recorrência |
| `recorrencia.solicitacao` | O pedido de confirmação foi enviado ao pagador ou cancelado por você | Solicitação |
| `cobranca_recorrente.criada` | Uma cobrança recorrente foi criada | Cobrança recorrente |
| `cobranca_recorrente.atualizada` | O status da cobrança recorrente mudou | Cobrança recorrente |
| `cobranca_recorrente.tentativa` | Uma tentativa de liquidação é nova ou mudou | Cobrança recorrente, com `tentativa` |

## Formato de cada `data`

Os exemplos de cobrança, Pix, devolução solicitada, recorrência, solicitação e
cobrança recorrente vêm de entregas reais do sandbox. O sandbox não gera
`devolucao.atualizada` nem `certificado.expirando`, cujos exemplos seguem os mesmos
campos do código.

### Cobrança

`tipo` é `cob` ou `cobv`.

```json
{ "tipo": "cobv", "txid": "cobvcobvcobvcobvcobvcobvcobv01", "contaId": "sandbox", "status": "CONCLUIDA", "valor": "120.00", "revisao": 1 }
```

### Pix

`txid` falta quando o Pix não veio de uma cobrança. `divergente` só aparece, como
`true`, quando o valor pago difere do cobrado.

```json
{ "endToEndId": "E00000000202610301314HTYSULD0GHI", "contaId": "sandbox", "txid": "pC164aMUZDaAroOn4tEArhtYsUlD0Ghi", "valor": "1.00", "horario": "2026-10-30T13:14:52.605742417Z", "divergente": true }
```

### Devolução

`status` é `EM_PROCESSAMENTO`, `DEVOLVIDO` ou `NAO_REALIZADO`. `motivo` vem quando a
devolução não foi realizada.

```json
{ "endToEndId": "E00000000202610301319TKZXDPSLN0B", "id": "dev1", "contaId": "sandbox", "valor": "10.00", "natureza": "ORIGINAL", "descricao": "Troca", "status": "DEVOLVIDO", "rtrId": "D000000002026103013198QFRgqhQWf5", "revisao": 0 }
```

### Certificado

`diasRestantes` é arredondado para cima e fica zero ou negativo depois de vencer. O
evento nunca leva o certificado.

```json
{ "contaId": "minha-loja", "sujeito": "CN=loja.example.com,O=Minha Loja", "impressaoDigital": "3f1a...", "validade": "2026-11-18T12:00:00Z", "diasRestantes": 20, "vencido": false }
```

### Recorrência

`motivoRejeicao` vem na rejeição.

```json
{ "idRec": "RR0000000020261030ipXnfCby6fT", "contaId": "sandbox", "status": "APROVADA", "contrato": "63100862", "periodicidade": "MENSAL", "dataInicial": "2026-11-02" }
```

### Solicitação

```json
{ "idSolicRec": "SC0000000020261030RNSLu0f4CYj", "idRec": "RR0000000020261030ipXnfCby6fT", "contaId": "sandbox", "status": "ENVIADA", "expiraEm": "2026-11-10T00:00:00Z" }
```

### Cobrança recorrente

`tentativa` só vem em `cobranca_recorrente.tentativa`. `motivoEncerramento` vem na
rejeição e no cancelamento.

```json
{ "txid": "cobrcobrcobrcobrcobrcobrcobr01", "idRec": "RR0000000020261030ipXnfCby6fT", "contaId": "sandbox", "status": "CONCLUIDA", "valor": "89.90", "vencimento": "2026-11-05", "revisao": 2, "tentativa": { "dataLiquidacao": "2026-11-05", "tipo": "AGND", "status": "PAGA" } }
```

A regra completa é a [RN-32](/gerado/regras-de-negocio/).
