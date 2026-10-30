# 🟧 Banco Inter

Receber Pix pela sua conta **Inter Empresas**, com o Vintém no meio.

> ⚠️ **Escrito pela documentação oficial, ainda não testado no sandbox do Inter.**
> O adapter segue o `swagger-api-pix.yaml` e o `swagger-token.yaml` do
> [portal do desenvolvedor](https://developers.inter.co/references/pix) e passa
> na suíte de conformidade contra um Inter simulado. Use no sandbox antes de
> qualquer conta de produção, e conte na [#28](https://github.com/vintemhq/vintem/issues/28)
> o que encontrar.

## 1. Integração e credenciais

1. No **Internet Banking do Inter Empresas**, crie uma integração de API (o
   portal do desenvolvedor descreve o caminho; os nomes da tela podem mudar).
2. Marque os escopos que o Vintém usa: `cob.read`, `cob.write`, `cobv.read`,
   `cobv.write`, `pix.read`, `pix.write` (devolução), `webhook.read` e
   `webhook.write`. O Vintém pede todos no token; faltando algum, o token é
   recusado.
3. Baixe o **Client ID**, o **Client Secret** e o **certificado**. O Inter
   entrega o certificado em dois arquivos, `.crt` e `.key`. Junte os dois num
   PEM só:

   ```sh
   cat Inter_API_Certificado.crt Inter_API_Chave.key > inter.pem
   ```

O sandbox (`cdpj-sandbox.partners.uatinter.co`) é o ambiente `homologacao` do
Vintém; produção é `cdpj.partners.bancointer.com.br`.

## 2. Cadastrar a conta no Vintém

Segredos vêm de arquivo, nunca de flag, porque flag fica no histórico do shell:

```sh
printf '%s' 'SEU_CLIENT_SECRET' > secret.txt   # ou crie o arquivo num editor
vintem conta criar --id minha-loja --nome "Minha Loja" --psp inter \
  --chave sua-chave-pix --ambiente homologacao \
  --client-id SEU_CLIENT_ID --client-secret-arquivo secret.txt \
  --certificado inter.pem
```

O certificado é conferido na hora (RN-62). Depois, apague `secret.txt` e o
`inter.pem`: o Vintém guarda tudo cifrado com a `VINTEM_MASTER_KEY`.

**Uma integração por conta corrente.** Se a integração estiver ligada a mais de
uma conta, o Inter exige o cabeçalho `x-conta-corrente`, que o adapter ainda não
envia.

## 3. Webhook

```sh
vintem conta webhook minha-loja --url-publica https://seu-vintem.exemplo.com
```

O Vintém gera uma URL com um token secreto e a registra na sua chave Pix
(`PUT /webhook/{chave}`). Se a entrega falhar, o Inter tenta de novo 4 vezes,
em 20, 30, 60 e 120 minutos; a conciliação do Vintém recupera o que passar
disso.

## Limites que importam

- **Token:** 5 por minuto. O Vintém reusa o token por uma hora, então isso só
  aparece se várias instâncias dividirem a mesma integração.
- **API Pix:** 120 chamadas por minuto em produção e 10 no sandbox, por rota.
  No sandbox, a conciliação e muitas cobranças seguidas batem nesse limite
  rápido.

## Testar no sandbox, com pagamento

O sandbox do Inter tem `POST /sandbox/cob/pagamento`, que paga uma cobrança
pelo BR Code. O adapter expõe isso como `SimularPagamento` (só em
homologação), e a suíte de conformidade usa para gravar as fitas com
pagamento e devolução: veja o comentário do `TestConformidadeGravada` em
`internal/adapters/psp/inter`.

## Desvios do Inter em relação à spec

Documentados e tratados em `internal/adapters/psp/inter/doc.go`: txid e id de
devolução repetidos sem erro documentado (o adapter consulta antes), filtro
`txId` no `GET /pix`, valor numérico na simulação de pagamento.
