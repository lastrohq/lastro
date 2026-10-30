---
title: Devoluções
description: Como devolver um Pix recebido, os limites e os estados de uma devolução.
---

Devolver é mandar de volta ao pagador todo ou parte de um Pix que você recebeu.

```bash
curl -X PUT http://localhost:8080/pix/$E2EID/devolucao/dev1 \
  -H "Authorization: Bearer $VINTEM_TOKEN" -H "Content-Type: application/json" \
  -d '{"valor":"10.00","descricao":"Troca"}'
```

```json
{
  "descricao": "Troca",
  "horario": {
    "liquidacao": "2026-10-30T13:14:56.659414735Z",
    "solicitacao": "2026-10-30T13:14:56.659152671Z"
  },
  "id": "dev1",
  "natureza": "ORIGINAL",
  "rtrId": "D00000000202610301314TPQm0zLiy3u",
  "status": "DEVOLVIDO",
  "valor": "10.00"
}
```

O `$E2EID` é o `endToEndId` do Pix, que vem no evento `pix.recebido`. O `id` é
definido por você, com 1 a 35 caracteres `[a-zA-Z0-9]`.

## Limites

Tudo isso é conferido antes de chamar o banco:

- **Saldo:** a soma das devoluções de um Pix não passa do valor recebido. Devolução
  `NAO_REALIZADO` não conta, porque o valor dela volta a estar disponível.
- **Prazo:** até 90 dias depois do pagamento.
- **Natureza:** só `ORIGINAL`, o padrão. `RETIRADA` é para Pix Saque e Troco, que o
  Vintém não recebe, e é recusada.
- **Descrição:** até 140 caracteres.

Fora dos limites, a resposta é [`RequisicaoInvalida`](/erros/requisicaoinvalida/):

```json
{
  "type": "https://vintem.dev/erros/RequisicaoInvalida",
  "title": "A requisição viola uma regra de negócio",
  "status": 400,
  "detail": "devolução inválida: devolução de 100.00 passa do saldo de 39.90 do Pix (recebido 49.90)"
}
```

O Pix precisa estar gravado no Vintém, pelo webhook ou pela conciliação, e ser da
conta da API key, se ela for restrita. Caso contrário, a resposta é
[`NaoEncontrado`](/erros/naoencontrado/). Conta desativada continua devolvendo,
porque o Pix já entrou.

## Repetir é seguro

A devolução é identificada pelo par `e2eid` + `id`. Repetir o par com o mesmo valor
devolve a devolução que já existe, sem chamar o banco. Com outro valor, a resposta é
**409** [`JaExiste`](/erros/jaexiste/). Devoluções simultâneas do mesmo Pix são
feitas uma de cada vez, para que juntas não passem do valor.

## Estados

```
EM_PROCESSAMENTO ──▶ DEVOLVIDO
                 └─▶ NAO_REALIZADO
```

O banco é a fonte da verdade. O resultado chega junto com o aviso de Pix, ou a
[conciliação](/conceitos/recebimentos-e-conciliacao/) o encontra consultando cada
devolução ainda `EM_PROCESSAMENTO`. Assim, o resultado da devolução de um Pix antigo
também aparece sem aviso.

## Eventos

| Evento | Quando |
|---|---|
| `devolucao.solicitada` | O banco aceitou a devolução. Se ele falhar, nada é gravado nem publicado |
| `devolucao.atualizada` | O banco informou o resultado. Aviso repetido não publica de novo |

:::note
O sandbox responde `DEVOLVIDO` na hora, então `devolucao.atualizada` não aparece nele.
:::

## Escopos

`PUT` exige `devolucao:write`, e `GET /pix/{e2eid}/devolucao/{id}` exige `pix:read`.

As regras completas são a [RN-20 e a RN-21](/gerado/regras-de-negocio/).
