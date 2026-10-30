---
title: Como funciona
description: O caminho de uma cobrança, da sua aplicação ao banco e de volta, e o que o Vintém guarda.
---

O Vintém é um binário que roda na sua infraestrutura, entre a sua aplicação e a
API Pix do seu banco. Sua aplicação fala uma API só, e o Vintém traduz para cada
banco.

```
Sua aplicação ──API do Vintém──▶ Vintém ──mTLS + OAuth2──▶ API Pix do banco
Sua aplicação ◀──evento assinado── Vintém ◀──webhook e consultas── API Pix do banco
```

## O caminho de uma cobrança

1. Sua aplicação cria a cobrança na API do Vintém.
2. O Vintém acha a conta dona da chave Pix e cria a cobrança no banco dela, pelo
   adapter daquele banco.
3. A resposta traz o `pixCopiaECola`, que o pagador usa para pagar no app dele.
4. O pagamento cai direto na sua conta. O banco avisa o Vintém pelo webhook.
5. O Vintém grava o Pix, conclui a cobrança e publica os eventos.
6. Sua aplicação recebe os eventos, assinados, na URL que você cadastrou.

Se o aviso do passo 4 se perder, a [conciliação](/conceitos/recebimentos-e-conciliacao/)
encontra o pagamento consultando o banco.

## As peças

| Peça | O que faz | Para saber mais |
|---|---|---|
| **API** | Rotas no formato da spec do BCB, mais extensões como `Idempotency-Key` | [Referência da API](/api/) |
| **Adapter** | Traduz para cada banco, com mTLS e OAuth2. Desvio de um banco fica dentro do adapter dele | [Contas e certificados](/conceitos/contas-e-certificados/) |
| **Webhook de entrada** | Recebe os avisos do banco, numa URL por conta | [Recebimentos e conciliação](/conceitos/recebimentos-e-conciliacao/) |
| **Conciliação** | Consulta o banco de tempos em tempos e recupera o que não foi avisado | [Recebimentos e conciliação](/conceitos/recebimentos-e-conciliacao/) |
| **Webhook de saída** | Entrega os eventos à sua aplicação, com assinatura, retry e DLQ | [Webhooks de saída](/conceitos/webhooks-de-saida/) |
| **Painel e MCP** | Telas e ferramentas para agentes, os dois sobre a mesma API | [Painel](/integrar/painel/) e [MCP](/integrar/mcp/) |

## Onde ficam os dados

Tudo fica num arquivo SQLite (`vintem.db`, por padrão), na sua máquina.
Certificados, chaves privadas e client secrets são cifrados com AES-256-GCM,
usando a `VINTEM_MASTER_KEY`. Sem essa chave o Vintém não sobe e as credenciais
não podem ser recuperadas.

## O Vintém não toca no dinheiro

O Vintém nunca guarda, recebe nem movimenta dinheiro. Ele usa as suas credenciais
para falar com o seu banco, e o Pix cai direto na sua conta. Por isso não há taxa
de intermediário: você paga só o que o seu banco cobra.

:::note
A tese de que software não custodial dispensa autorização do BCB é uma inferência,
não uma norma. Nada aqui é aconselhamento regulatório. A
[ADR-004](/gerado/adr/004-nao-custodial/) explica a decisão.
:::

## Para ir além

Esta seção resume e explica. As regras completas, com número e motivo, estão nas
[regras de negócio](/gerado/regras-de-negocio/).
