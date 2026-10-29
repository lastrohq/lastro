<div align="center">

# Lastro

### Pix sem gateway: 0% de intermediação, sua conta PJ, sua infraestrutura.

**Gateway self-hosted e multi-banco para Pix, com Pix Automático desde o primeiro dia.**
Uma imagem Docker que fala a API Pix do Banco Central, traz um adaptador por banco,
cuida dos certificados mTLS e normaliza os webhooks.

[English](./README.md) · [Docs](https://lastro.dev/docs) · [Board](https://github.com/orgs/lastrohq/projects)

</div>

> 🚧 **Pré-alpha.** Nada aqui funciona ainda. Acompanhe pelo board.

## Por quê

Gateways cobram a partir de ~1,19% por Pix — num meio de pagamento que custa
quase nada para a PJ no próprio banco. Todo banco já expõe a
[API Pix do BCB](https://github.com/bacen/pix-api), mas cada um desvia dela do
seu jeito, exige o seu ritual de certificado mTLS e manda webhook no seu formato.

O Lastro é a camada que falta: você roda, pluga as credenciais do **seu** banco,
e a sua aplicação fala com **uma** API só, qualquer que seja o banco que recebe.

## O que o MVP vai fazer

- **Uma API, vários bancos** — Efí, Inter, Sicoob e Banco do Brasil primeiro; adapters abertos à comunidade.
- **Cobranças** — imediata (`cob`) e com vencimento (`cobv`), com BR Code / QR embutido.
- **Pix Automático** — `rec`, `solicrec`, `cobr` e webhooks de recorrência.
- **Webhook que não se perde** — normalizado, assinado, com retry e um job de conciliação que recupera pagamento cujo aviso sumiu.
- **Sandbox com estado** — teste o fluxo inteiro sem conta PJ.
- **Binário único** — `docker run` e pronto; SQLite por padrão.
- **Use com seu agente** — servidor MCP embutido.

## Bancos

| Banco | `--psp` | cob | cobv | Devolução | Webhook | Situação |
|---|---|---|---|---|---|---|
| Sandbox do Lastro | `sandbox` | ✅ | ✅ | ✅ | ✅ | Para testar sem conta PJ ([guia](./docs/SANDBOX.md)) |
| Efí | `efi` | ✅ | ✅ | ✅ | ✅ | **Ainda não testado na homologação do Efí** ([guia](./docs/EFI.md)) |
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
