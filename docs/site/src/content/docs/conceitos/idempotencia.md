---
title: Idempotência
description: Como repetir uma requisição sem criar nada em dobro, com o header Idempotency-Key.
---

Numa queda de rede, sua aplicação não sabe se a cobrança foi criada. Com a
`Idempotency-Key`, ela pode repetir a requisição: a cobrança é criada uma vez só, e
a repetição recebe a mesma resposta.

```bash
curl -i -X POST http://localhost:8080/cob \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Idempotency-Key: pedido-42" \
  -H "Content-Type: application/json" \
  -d '{"valor":{"original":"20.00"},"chave":"sandbox@vintem.dev"}'
```

Na segunda vez, a resposta é a original, byte a byte, com um header a mais:

```http
HTTP/1.1 201 Created
Idempotent-Replayed: true
```

Use uma chave que represente a operação, como o id do pedido. Uma chave nova a cada
tentativa não protege de nada.

## Onde vale

O header é aceito em todo `POST`, `PUT` e `PATCH`.

| Rota | Header |
|---|---|
| `POST /cob`, `POST /rec`, `POST /solicrec` e `POST /cobr` | **Obrigatório**: o identificador é gerado pelo servidor |
| `PUT /cob/{txid}`, `PUT /cobv/{txid}` e `PUT /cobr/{txid}` | Opcional: o txid já identifica a criação |
| Os outros `POST`, `PUT` e `PATCH` | Opcional |

Sem o header numa rota que o exige, a resposta é
[`RequisicaoInvalida`](/erros/requisicaoinvalida/).

## O que acontece em cada caso

| Situação | Resposta |
|---|---|
| Mesma chave, mesma requisição, em até 24 h | A resposta original, com `Idempotent-Replayed: true`, sem chamar o banco de novo |
| Mesma chave, requisição diferente | **409** [`IdempotencyKeyReutilizada`](/erros/idempotencykeyreutilizada/) |
| Mesma chave, com a primeira ainda em andamento | **409** [`RequisicaoEmAndamento`](/erros/requisicaoemandamento/), com `Retry-After: 1` |
| Primeira resposta foi 5xx | Nada foi guardado: a repetição executa de novo |
| Corpo acima de 1 MiB | **413** [`CorpoGrandeDemais`](/erros/corpograndedemais/) |

"Mesma requisição" compara o método, o caminho e o corpo.

## Detalhes que importam

- **A chave pertence à API key.** Duas API keys podem usar a mesma chave sem colidir.
- **Resposta 5xx não é guardada.** O motivo de mandar a chave é poder tentar de novo.
  Respostas 4xx são guardadas: corrija a requisição e use outra chave.
- **Reserva abandonada é liberada.** Se o processo cair no meio, a reserva sem
  resposta por mais de 2 minutos deixa de valer.

A regra completa é a [RN-11](/gerado/regras-de-negocio/).
