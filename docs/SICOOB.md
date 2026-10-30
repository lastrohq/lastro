# 🟩 Sicoob

Receber Pix pela sua conta **Sicoob** (cooperado PJ), com o Vintém no meio.

> ⚠️ **Escrito pela spec do BCB e por relatos públicos, ainda não testado com
> uma conta de cooperado.** O portal [Sicoob Developers](https://developers.sicoob.com.br)
> só mostra o swagger com login. O adapter segue a spec da API Pix do BCB, os
> desvios que o projeto [kmee/brbank](https://github.com/kmee/brbank) registrou
> no sandbox do Sicoob e passa na suíte de conformidade contra um Sicoob
> simulado. Comece com uma cobrança de centavos e conte na
> [#29](https://github.com/vintemhq/vintem/issues/29) o que encontrar.

## 1. Aplicação e credenciais

1. Entre no **portal Sicoob Developers** com o acesso da cooperativa e crie
   uma aplicação para a API **Pix Recebimentos**.
2. Cadastre na aplicação o **certificado e-CNPJ A1** da empresa (o mesmo
   `.pfx` que você usa em outros sistemas). É ele que identifica a aplicação:
   o Sicoob não tem client secret.
3. Libere os escopos que o Vintém usa: `cob.read`, `cob.write`, `cobv.read`,
   `cobv.write`, `pix.read`, `pix.write` (devolução), `webhook.read` e
   `webhook.write`. O Vintém pede todos no token; faltando algum, o token é
   recusado.
4. Anote o **Client ID** da aplicação.

**Não há homologação com estado.** O sandbox do Sicoob aceita qualquer token,
não usa o certificado e responde dados aleatórios, sem guardar as cobranças.
Por isso a conta do Sicoob no Vintém é sempre `--ambiente producao`; para
testar o fluxo sem banco, use o [sandbox do Vintém](./SANDBOX.md).

## 2. Cadastrar a conta no Vintém

A senha do certificado vem de arquivo, nunca de flag, porque flag fica no
histórico do shell:

```sh
printf '%s' 'SENHA_DO_PFX' > senha.txt   # ou crie o arquivo num editor
vintem conta criar --id minha-loja --nome "Minha Loja" --psp sicoob \
  --chave sua-chave-pix --ambiente producao \
  --client-id SEU_CLIENT_ID \
  --certificado ecnpj.pfx --senha-certificado-arquivo senha.txt
```

Não precisa de `--client-secret-arquivo`. O certificado é conferido na hora
(RN-62). Depois, apague `senha.txt`: o Vintém guarda tudo cifrado com a
`VINTEM_MASTER_KEY`.

## 3. Webhook

```sh
vintem conta webhook minha-loja --url-publica https://seu-vintem.exemplo.com
```

O Vintém gera uma URL com um token secreto e a registra na sua chave Pix
(`PUT /webhook/{chave}`). Se um aviso se perder, a conciliação do Vintém
recupera o Pix em até 5 minutos.

## Primeiro teste

1. Crie uma cobrança de R$ 0,01 e pague pelo app de outro banco.
2. Confira que ela aparece como `CONCLUIDA` no painel (pelo webhook ou pela
   conciliação).
3. Devolva o centavo pelo painel ou pela API.

Para gravar a suíte de conformidade contra o Sicoob (sem pagar nada; os
cenários de pagamento ficam de fora), veja o comentário do
`TestConformidadeGravada` em `internal/adapters/psp/sicoob`.

## Desvios do Sicoob em relação à spec

Documentados e tratados em `internal/adapters/psp/sicoob/doc.go`: token só com
`client_id` (sem secret) em `auth.sicoob.com.br`, header `client_id` em toda
chamada, BR Code no campo `brcode`, erros do gateway num formato próprio, e
txid e id de devolução repetidos sem erro documentado (o adapter consulta
antes).
