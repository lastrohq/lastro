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

  O aviso pelo webhook só chega quando `lastro conta webhook` e o `serve` rodam
  no mesmo processo, porque o sandbox guarda a URL em memória. Entre processos,
  que é o caso normal, a **conciliação** encontra a resposta na rodada seguinte
  (5 min por padrão, ou `lastro conciliar` na hora), e o resultado é o mesmo.
- **Estado:** o sandbox lembra das cobranças depois de reiniciar o Lastro. Ele
  reconstrói a partir das cobranças que o próprio Lastro gravou no SQLite: uma
  cobrança criada antes do restart é paga no horário de sempre, e os valores
  mágicos continuam valendo. As **devoluções** também: as que o Lastro gravou
  contam no saldo do Pix e respondem pelo id depois do restart.
- **Devolução:** `PUT /pix/{e2eid}/devolucao/{id}` devolve um Pix que o Lastro
  já gravou (pelo webhook ou pela conciliação). O sandbox responde `DEVOLVIDO`
  na hora, com `rtrId` e horário de liquidação. Por isso o evento
  `devolucao.atualizada`, que sai quando o banco informa o resultado depois,
  não aparece no sandbox.
- **Valor da cobv:** o sandbox paga o valor **original**. Multa, juros e desconto
  são calculados pelo banco de verdade.
- **Cobrança recorrente:** `PUT /cobr/{txid}` (ou `POST /cobr`) cobra um ciclo
  de uma recorrência aprovada. O sandbox comprime o calendário: em vez de esperar
  o dia da liquidação, a cobrança anda um passo a cada atraso (5 s). Depois de um
  atraso o PSP do pagador aceita a instrução (`ATIVA`); depois de dois, a
  tentativa do vencimento é liquidada. Como a cobrança tem o valor fixo da
  recorrência, quem escolhe o comportamento é a **recorrência**:

  | Valor fixo | A cobrança |
  |---|---|
  | R$ 0,44 | o PSP do pagador rejeita a instrução: `REJEITADA` |
  | R$ 0,55 | a tentativa do vencimento falha por falta de saldo (`AM04`) e a cobrança segue `ATIVA`; a retentativa (`POST /cobr/{txid}/retentativa/{data}`) é paga um atraso depois de pedida |
  | R$ 0,66 | o sandbox recusa criar |
  | outros | é paga no vencimento: `CONCLUIDA` |

  Cancelar (`PATCH /cobr/{txid}` com `CANCELADA`) congela a cobrança: nada é
  liquidado depois. A cada passo, o sandbox avisa pelo webhook de cobrança
  recorrente (`/webhooks/cobr/{conta}/{token}`), que `lastro conta webhook`
  registra junto com os outros. O `GET /cobr/{txid}` também pergunta ao sandbox
  enquanto a cobrança está aberta, e a conciliação confere as abertas.

  ⚠️ Como na recorrência, a URL do webhook de cobrança recorrente fica **em
  memória**: entre processos o aviso não chega, e a mudança aparece na consulta
  ou na próxima conciliação.
