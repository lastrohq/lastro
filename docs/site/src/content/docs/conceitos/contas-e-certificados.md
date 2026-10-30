---
title: Contas e certificados
description: O que é uma conta recebedora, as credenciais de cada banco e como o Vintém confere o certificado.
---

Uma **conta recebedora** é a combinação de um banco, uma chave Pix, as credenciais
e um ambiente (`homologacao` ou `producao`). Uma instância atende várias contas, e
toda cobrança pertence a exatamente uma: a dona da `chave` informada no corpo.

- O id da conta usa `[a-z0-9-]`, com até 63 caracteres, e não muda. O banco também não.
- A chave Pix é única na instância.
- Sem `--ambiente`, a conta é criada em `homologacao`.

## Cadastrar uma conta

```bash
vintem conta criar --id minha-loja --nome "Minha Loja" --psp efi \
  --chave sua-chave-pix@exemplo.com --ambiente homologacao \
  --client-id SEU_CLIENT_ID --client-secret-arquivo secret.txt \
  --certificado homologacao.p12
```

Segredos vêm de arquivo, nunca de flag, porque flag fica no histórico do shell.
Depois do cadastro, apague os arquivos: o Vintém guarda tudo cifrado. A conta
também pode ser criada por `POST /vintem/v1/contas`, com o escopo `conta:admin`.

## O que cada banco pede

| Banco (`--psp`) | Client secret | Certificado | Particularidade | Guia |
|---|---|---|---|---|
| Efí (`efi`) | Sim | `.p12` | | [Efí](/gerado/efi/) |
| Inter (`inter`) | Sim | PEM, com certificado e chave no mesmo arquivo | Uma integração por conta corrente | [Inter](/gerado/inter/) |
| Sicoob (`sicoob`) | Não: o certificado identifica a aplicação | Sim | | [Sicoob](/gerado/sicoob/) |
| Banco do Brasil (`bb`) | Sim | Só em produção | Exige `--chave-aplicacao`, a `gw-dev-app-key` do portal | [Banco do Brasil](/gerado/bb/) |
| Sandbox (`sandbox`) | Não | Não | Banco de mentira, com estado | [Sandbox](/gerado/sandbox/) |

## O que cada banco oferece

Cada adapter declara as próprias capacidades. Pedir algo que o banco da conta não
oferece responde **501** [`NaoSuportado`](/erros/naosuportado/), com o nome da
capacidade.

| Capacidade | Efí | Inter | Sicoob | BB | Sandbox |
|---|---|---|---|---|---|
| `cob` e `cobv` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `devolucao` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `webhook` | ✓ | ✓ | ✓ | ✓ | ✓ |
| `rec` e `cobr` (Pix Automático) | | | | | ✓ |

## O certificado é conferido no cadastro

O erro de certificado aparece na hora do cadastro, não na primeira cobrança. O
Vintém aceita `.p12` com senha, ou PEM com certificado e chave no mesmo arquivo, e
recusa, com o motivo e sem mostrar o conteúdo:

- arquivo ilegível;
- senha errada;
- chave privada que não corresponde ao certificado;
- chave PEM cifrada;
- certificado vencido;
- certificado já cadastrado numa conta de homologação, se a conta nova for de produção.

Sujeito, impressão digital e validade ficam visíveis em `vintem conta listar` e em
`GET /vintem/v1/contas`. O certificado e a chave continuam cifrados.

## Aviso de vencimento

Quando faltam 30 dias ou menos para o certificado de uma conta ativa vencer, o
Vintém publica o evento [`certificado.expirando`](/conceitos/webhooks-de-saida/#catálogo-de-eventos),
uma vez por certificado. O painel mostra o mesmo alerta na tela de contas.

A verificação roda ao subir e depois a cada hora (`VINTEM_CERTIFICADO_INTERVALO`;
`0` desliga). Como em todo evento, se não houver destino cadastrado na hora, o
aviso é descartado e não se repete.

## Desativar uma conta

```bash
vintem conta desativar minha-loja
```

Conta desativada recusa cobranças novas com [`ContaInativa`](/erros/containativa/).
O que já existe continua valendo: as cobranças antigas ainda recebem webhook, entram
na conciliação e os Pix já recebidos ainda podem ser devolvidos.
