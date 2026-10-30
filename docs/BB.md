# 🟨 Banco do Brasil

Receber Pix pela sua conta PJ do **Banco do Brasil**, com o Lastro no meio.

> ⚠️ **Escrito pela spec do BCB e por relatos públicos, ainda não testado pelo
> Lastro na homologação do BB.** O adapter segue a API Pix v2 do
> [Portal Developers BB](https://developers.bb.com.br), o que o projeto
> [kmee/brbank](https://github.com/kmee/brbank) registrou ao rodar cob e cobv
> na homologação do BB, e passa na suíte de conformidade contra um BB
> simulado. Teste na homologação antes de produção e conte na
> [#30](https://github.com/lastrohq/lastro/issues/30) o que encontrar.

## 1. Aplicação e credenciais

1. No **Portal Developers BB**, crie uma aplicação e inclua a **API Pix**.
2. Libere os escopos que o Lastro usa: `cob.read`, `cob.write`, `cobv.read`,
   `cobv.write`, `pix.read`, `pix.write` (devolução), `webhook.read` e
   `webhook.write`. O Lastro pede todos no token; sem eles o BB recusa com
   `invalid_scope`.
3. Baixe as credenciais da aplicação. São três: **Client ID**, **Client
   Secret** e **chave de aplicação** (`appKey`, a `developer_application_key`).
   A chave de aplicação vai na URL de toda chamada.

**Homologação.** A aplicação "Em teste" usa `api.hm.bb.com.br`, que não pede
certificado. A homologação é compartilhada por todas as aplicações e recebe na
chave Pix de teste que o BB publica, `9e881f18-cc66-4fc7-8f2c-a795dbb2bfc1`.
Ela não dispara webhook; para pagar uma cobrança, o BB tem um simulador (veja
abaixo).

**Produção.** A API de produção (`api-pix.bb.com.br`) exige mTLS com um
certificado A1 ICP-Brasil, de preferência o e-CNPJ, cadastrado na aplicação.
As credenciais de produção saem do portal quando a aplicação vai para produção,
ou com o gerente da conta.

## 2. Cadastrar a conta no Lastro

Na homologação, sem certificado:

```sh
printf '%s' 'SEU_CLIENT_SECRET' > secret.txt   # ou crie o arquivo num editor
lastro conta criar --id bb-teste --nome "BB Homologação" --psp bb \
  --chave 9e881f18-cc66-4fc7-8f2c-a795dbb2bfc1 --ambiente homologacao \
  --client-id SEU_CLIENT_ID --client-secret-arquivo secret.txt \
  --chave-aplicacao SUA_APP_KEY
```

Em produção, com a sua chave Pix e o certificado:

```sh
lastro conta criar --id minha-loja --nome "Minha Loja" --psp bb \
  --chave sua-chave-pix --ambiente producao \
  --client-id SEU_CLIENT_ID --client-secret-arquivo secret.txt \
  --chave-aplicacao SUA_APP_KEY \
  --certificado ecnpj.pfx --senha-certificado-arquivo senha.txt
```

O certificado é conferido na hora (RN-62). Depois, apague `secret.txt` e
`senha.txt`: o Lastro guarda tudo cifrado com a `LASTRO_MASTER_KEY`.

## 3. Webhook

```sh
lastro conta webhook minha-loja --url-publica https://seu-lastro.exemplo.com
```

O Lastro gera uma URL com um token secreto e a registra na sua chave Pix
(`PUT /webhook/{chave}`). O BB chama o webhook com o certificado de cliente
dele: o proxy na frente do Lastro precisa de um certificado de AC confiável.
Se um aviso se perder, a conciliação do Lastro recupera o Pix em até 5 minutos.

## Testar na homologação, com pagamento

O simulador do BB (`POST /testes-portal-desenvolvedor/v1/boletos-pix/pagar`)
paga uma cobrança pelo BR Code e a conclui em até uns 20 segundos. O adapter
expõe isso como `SimularPagamento` (só em homologação), e a suíte de
conformidade usa para gravar as fitas com pagamento e devolução: veja o
comentário do `TestConformidadeGravada` em `internal/adapters/psp/bb`.

## Desvios do BB em relação à spec

Documentados e tratados em `internal/adapters/psp/bb/doc.go`: chave de
aplicação na query (`gw-dev-app-key` na homologação, `gw-app-key` em
produção), BR Code em `textoImagemQRcode`, erros do gateway em lista, e txid e
id de devolução repetidos sem erro documentado (o adapter consulta antes).
