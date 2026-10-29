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
lastro apikey criar --nome teste --escopo cob:read --escopo cob:write   # copie o token lst_...
export LASTRO_TOKEN=lst_...
lastro serve &

curl -X PUT localhost:8080/cob/pedido00000000000000000000001 \
  -H "Authorization: Bearer $LASTRO_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"calendario":{"expiracao":3600},"valor":{"original":"49.90"},"chave":"loja@example.com"}'
# → 201, status ATIVA, com o pixCopiaECola

sleep 5
curl -H "Authorization: Bearer $LASTRO_TOKEN" localhost:8080/cob/pedido00000000000000000000001
# → status CONCLUIDA, revisao 1
```

Você nem precisa consultar: a **conciliação** (a cada 5 min no `serve`, ou na
hora com `lastro conciliar`) encontra o pagamento do sandbox e conclui a cobrança
sozinha. A consulta também pergunta ao PSP quando a cobrança ainda está `ATIVA`.

## Como ele se comporta

Toda cobrança (cob ou cobv) é **paga 5 segundos depois de criada**, desde que
não tenha expirado antes. O pagamento aparece na consulta da cobrança (`CONCLUIDA`) e em
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

- **Webhook do banco:** o sandbox não chama o webhook de entrada sozinho; o
  pagamento chega pela conciliação. Para testar o webhook, simule o banco com
  `curl -X POST {url-do-webhook}/pix` (veja a RN-30).
- **Pix Automático:** `POST /rec` cria a recorrência e `POST /solicrec` pede a
  confirmação. Depois do atraso, o "pagador" responde e o sandbox avisa pelo
  webhook de recorrência, conforme o **valor fixo** da recorrência:

  | Valor fixo | O pagador |
  |---|---|
  | R$ 0,66 | — o sandbox recusa criar a recorrência |
  | R$ 0,77 | rejeita (`AP13`) |
  | R$ 0,88 | não responde: a solicitação expira |
  | outros (e valor variável) | aprova |

  ⚠️ O sandbox guarda a URL do webhook de recorrência **em memória**: o aviso só
  chega se `lastro conta webhook` e o `serve` rodarem no mesmo processo. Entre
  processos, a recorrência fica `CRIADA` até a conciliação de recorrências (#93).
- **Estado:** o sandbox lembra das cobranças depois de reiniciar o Lastro. Ele
  reconstrói a partir das cobranças que o próprio Lastro gravou no SQLite: uma
  cobrança criada antes do restart é paga no horário de sempre, e os valores
  mágicos continuam valendo. As **devoluções** do sandbox ainda ficam só em
  memória, até o Lastro gravar devoluções (F7).
- **Valor da cobv:** o sandbox paga o valor **original**. Multa, juros e desconto
  são calculados pelo banco de verdade.
- **Pix Automático:** entra com a fase dele.
