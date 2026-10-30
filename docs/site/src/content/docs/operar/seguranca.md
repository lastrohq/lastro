---
title: Segurança
description: A chave mestra, o que fica cifrado, as API keys, os certificados e o que o Vintém deixa para o seu proxy.
---

O Vintém nunca toca no dinheiro: o Pix cai direto na sua conta
([ADR-004](/gerado/adr/004-nao-custodial/)). O que ele guarda de sensível são as
credenciais do seu banco, que permitem criar cobranças e devolver Pix em seu nome.

## A chave mestra

`VINTEM_MASTER_KEY` tem 32 bytes em base64 e cifra as credenciais com AES-256-GCM.

- **Guarde fora do servidor,** num cofre de senhas. Sem ela, as credenciais não podem
  ser recuperadas.
- **Não a coloque no mesmo backup do banco.** Quem tiver os dois tem as credenciais.
- **Não a versione.** O `.env` com ela fica com permissão `600` e fora do git.

Sem a chave, o Vintém não sobe. Uma chave mal formada, ou diferente da que gravou os
dados, também é recusada ao abrir o banco, com o nome da conta ou do destino que não abriu.

### Trocar a chave mestra

O comando recifra as credenciais e os segredos numa transação só: ou tudo passa para a
chave nova, ou nada muda.

1. [Faça o backup](/operar/backup-e-atualizacao/#fazer-backup).
2. Gere a chave nova com `vintem chave gerar` e guarde em lugar seguro.
3. Pare o Vintém. Um servidor no ar continuaria com a chave antiga na memória.
4. Rode a troca:

```bash
docker compose stop vintem
docker compose run --rm -e VINTEM_MASTER_KEY_NOVA=a-chave-nova vintem chave trocar
```

5. Troque `VINTEM_MASTER_KEY` no `.env` e suba com `docker compose up -d vintem`.

Backups antigos continuam presos à chave antiga. Se a troca foi por vazamento, troque
também as credenciais no portal de cada banco: quem tinha a chave antiga e um backup
ainda consegue abri-lo.

## O que fica cifrado

| Dado | Como fica |
|---|---|
| Certificado, chave privada, client secret e senha do certificado | Cifrados com a chave mestra |
| Segredo `whsec_` dos destinos | Cifrado, porque é preciso tê-lo em claro para assinar |
| API keys | Só o hash SHA-256 |
| Token da URL de webhook | Só o hash |
| Sujeito, impressão digital e validade do certificado | Em claro: são dados públicos do certificado |

Nada disso vai para os logs, nem mascarado. Segredos entram por arquivo, nunca por flag,
porque flag fica no histórico do shell. Apague os arquivos depois do cadastro.

## API keys

- Dê a cada aplicação uma key própria, com o mínimo de [escopos](/conceitos/api-keys-e-escopos/)
  e restrita à conta dela.
- **Para rotacionar:** crie a key nova, troque na aplicação e revogue a antiga com
  `vintem apikey revogar <id>`. A revogação vale na hora.

## Certificados

O certificado é [conferido no cadastro](/conceitos/contas-e-certificados/#o-certificado-é-conferido-no-cadastro)
e gera um aviso 30 dias antes de vencer. Para um certificado de homologação não ir parar
em produção, a conta de produção recusa um certificado já cadastrado em homologação.

## Webhooks

- **Entrada:** o token na URL é o que autentica o banco, então trate a URL como segredo.
  Rodar `vintem conta webhook` de novo troca o token.
- **Saída:** confira sempre a [assinatura](/conceitos/webhooks-de-saida/#conferir-a-assinatura).
  Para trocar o segredo, crie um destino novo e remova o antigo.

## O que fica com o seu proxy

- **TLS:** o Vintém fala HTTP. Nunca exponha a porta dele direto na internet.
- **Limite de requisições:** o Vintém não tem. Se precisar, configure no proxy.
- **Restrição de origem:** o proxy pode limitar os webhooks aos IPs do banco, como o
  [guia do Efí](/gerado/efi/) recomenda, e o `/painel/` à sua rede.

O corpo das requisições tem limite de 1 MiB em todas as rotas. O `/health` é público e
mostra a versão.

## Reportar uma vulnerabilidade

Não abra issue pública. Siga a [política de segurança](https://github.com/vintemhq/vintem/blob/develop/SECURITY.md).
