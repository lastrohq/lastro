<div align="center">

# Lastro

### Pix sem gateway: sua conta no banco, sua infraestrutura, nenhuma taxa de intermediário.

**Gateway self-hosted e multi-banco para Pix, com Pix Automático.**
Um binário que fala a API Pix do Banco Central, traz um adaptador por banco,
cuida dos certificados mTLS e normaliza os webhooks.

<img src="docs/demo/sandbox.gif" alt="lastro sandbox: uma cobrança Pix é criada e paga em poucos segundos" width="820">

[English](./README.md) · [Docs](./docs) · [Board](https://github.com/orgs/lastrohq/projects/1)

</div>

> 🚧 **Alpha.** O sandbox e o adapter da Efí funcionam; a API ainda pode mudar. Acompanhe pelo board.

## Teste em 10 segundos

```bash
docker run --rm -p 8080:8080 ghcr.io/lastrohq/lastro sandbox
```

Ele imprime uma API key de teste e um `curl` que cria uma cobrança Pix. Cole no
terminal: cinco segundos depois o "pagador" de mentira paga e a cobrança vira
`CONCLUIDA`. Sem conta PJ, sem configurar nada. Sem Docker:
`go run github.com/lastrohq/lastro/cmd/lastro@latest sandbox`. Valores mágicos no [guia do sandbox](./docs/SANDBOX.md).

## Use com seu agente

`lastro mcp` é um servidor MCP para agentes de IA: cria, consulta, lista e
cancela cobranças Pix, e devolve pagamentos. Ele fala com a API do Lastro usando
uma API key, então os escopos da key valem: uma key só de leitura dá um agente
que só consulta.

```bash
lastro sandbox   # copie a key lst_... que ele imprime
claude mcp add lastro -e LASTRO_TOKEN=lst_... -e LASTRO_CHAVE_PIX=sandbox@lastro.dev -- lastro mcp
```

Depois peça: *"cobre R$ 49,90 do pedido 42 e me mostre o Pix copia e cola"*.
Qualquer cliente MCP funciona igual: rode `lastro mcp` por stdio com
`LASTRO_URL` (padrão `http://localhost:8080`), `LASTRO_TOKEN` e, se quiser,
`LASTRO_CHAVE_PIX`.

## Por quê

Gateways cobram cerca de 1% por Pix, e ainda são mais uma empresa entre você e
o seu dinheiro. Todo banco já expõe a
[API Pix do BCB](https://github.com/bacen/pix-api), mas cada um desvia dela do
seu jeito, exige o seu ritual de certificado mTLS e manda webhook no seu formato.

O Lastro é a camada que falta: você roda, pluga as credenciais do **seu** banco,
e a sua aplicação fala com **uma** API só, qualquer que seja o banco que recebe.

## Quanto você paga

O Lastro é gratuito e não cobra nada por Pix. Você paga a tarifa de Pix do
**seu** banco, e é só isso. Quanto você economiza depende do banco:

| Por onde o Pix passa | Taxa por Pix | Venda de R$ 50 | Venda de R$ 500 |
|---|---|---|---|
| [Stripe](https://stripe.com/br/pricing) (Pix, só por convite) | 1,19% | R$ 0,60 | R$ 5,95 |
| [Mercado Pago](https://www.mercadopago.com.br/ferramentas-para-vender/check-out) (checkout, recebimento na hora) | 0,99% | R$ 0,50 | R$ 4,95 |
| [Asaas](https://www.asaas.com/precos-e-taxas) | R$ 2,15 fixos (R$ 1,08 nos 3 primeiros meses) | R$ 2,15 | R$ 2,15 |
| **Lastro + [Efí Empresas](https://sejaefi.com.br/tarifas)** | 1,19% (tarifa da Efí; o Lastro soma 0) | R$ 0,60 | R$ 5,95 |

<sub>Páginas de preço públicas, consultadas em 29/10/2026. A taxa negociada da sua conta pode ser outra.</sub>

**Sem ilusão:** com a Efí, a taxa é a mesma da Stripe. O que você ganha hoje é
ser dono do processo: seus dados, seus webhooks, nenhuma amarra e a liberdade
de trocar de banco sem reescrever a integração. A taxa cai com bancos que
limitam ou fixam a tarifa de Pix da conta PJ; os adapters do Inter, do Sicoob e
do Banco do Brasil são os próximos do board.

## O que ele faz

- **Uma API, vários bancos** — Efí hoje; Inter, Sicoob e Banco do Brasil em seguida. Todo adapter passa pela mesma [suíte de conformidade](./CONTRIBUTING.md#suíte-de-conformidade).
- **Cobranças** — imediata (`cob`) e com vencimento (`cobv`), com BR Code / QR embutido.
- **Pix Automático** — `rec`, `solicrec`, `cobr` e webhooks de recorrência.
- **Webhook que não se perde** — normalizado, assinado, com retry e um job de conciliação que recupera pagamento cujo aviso sumiu.
- **Devoluções** — total ou parcial, nunca mais do que foi recebido.
- **Sandbox com estado** — teste o fluxo inteiro sem conta PJ.
- **Binário único** — `docker run` e pronto; SQLite por padrão.
- **Use com seu agente** — servidor MCP embutido (`lastro mcp`).

## Bancos

| Banco | `--psp` | cob | cobv | Devolução | Webhook | Situação |
|---|---|---|---|---|---|---|
| Sandbox do Lastro | `sandbox` | ✅ | ✅ | ✅ | ✅ | Para testar sem conta PJ ([guia](./docs/SANDBOX.md)) |
| Efí | `efi` | ✅ | ✅ | ✅ | ✅ | Testado na homologação da Efí (cobranças, pagamento, devolução); falta testar o cadastro do webhook ([guia](./docs/EFI.md)) |
| Inter, Sicoob, Banco do Brasil | — | | | | | Planejado |

## Não custodial

O Lastro nunca guarda nem movimenta dinheiro. Ele roda com as **suas**
credenciais contra a **sua** conta. Isto não é aconselhamento jurídico ou
regulatório.

## Documentação do projeto

| Documento | Para quê |
|---|---|
| [Regras de negócio](./docs/REGRAS-DE-NEGOCIO.md) | O que o Lastro garante e o que recusa |
| [Arquitetura](./docs/ARQUITETURA.md) | Hexagonal, pastas, padrões, testes |
| [Sandbox](./docs/SANDBOX.md) | Testar sem conta PJ, com valores mágicos |
| [Efí](./docs/EFI.md) | Ligar uma conta Efí: credenciais, certificado e webhook |
| [Design system](./docs/DESIGN-SYSTEM.md) | Tokens, componentes e telas do painel |
| [Manual do projeto](./docs/EQUIPE.md) | Board, fluxo, DoR e DoD |
| [ADRs](./docs/adr/) | Decisões registradas e o porquê |
| [Como contribuir](./CONTRIBUTING.md) | Inclusive como escrever um adapter de banco |

## Licença

[Apache-2.0](./LICENSE). "Pix" é marca do Banco Central do Brasil; o Lastro é
um projeto independente, sem vínculo nem endosso do BCB.
