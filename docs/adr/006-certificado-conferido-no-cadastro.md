# ADR-006 · Certificado do mTLS conferido no cadastro, com go-pkcs12

**Status:** Aceita · **Data:** 29/10/2026

## Contexto

Os bancos entregam o certificado do mTLS como .p12 com senha (Efí, Inter) ou
como PEM. Até aqui o Lastro guardava o arquivo como bytes, sem ler. Senha
errada, arquivo corrompido ou certificado vencido entravam sem erro e só
apareciam quando o banco recusasse a primeira chamada. Além disso, a validade
ficava dentro do blob cifrado, e sem ela não existe alerta de vencimento (RN-62).

A stdlib do Go lê PEM, mas não lê PKCS#12. O `golang.org/x/crypto/pkcs12` está
congelado e marcado como deprecated. Ele não lê os .p12 com AES, que são o
padrão do OpenSSL 3 e de quem exporta certificado hoje.

## Decisão

- **O certificado é lido no cadastro**, em `app.Contas.Criar`, que é o único
  caminho da CLI e da API. O Lastro recusa arquivo ilegível, senha errada, chave
  que não corresponde ao certificado e certificado vencido. Nenhuma mensagem
  traz o conteúdo do arquivo.
- **A leitura do .p12 usa [`software.sslmate.com/src/go-pkcs12`][lib]** (BSD-3,
  mantida, lê os formatos com AES e os antigos com RC2/3DES). A leitura do PEM usa
  a stdlib. Isso fica em `internal/platform/certificado`. O `app` chega lá pelo
  port `ports.Certificados`, porque `app` não importa `platform`.
- **Os dados públicos ficam fora do blob cifrado**, em colunas da conta: sujeito,
  impressão digital SHA-256 e validade. O certificado e a chave privada continuam
  só no blob (RN-61). A validade vai em segundos Unix, porque certificado pode
  valer até 9999-12-31 (RFC 5280), além do que cabe em nanossegundos `int64`.
- **Chave PEM cifrada é recusada** com orientação (exportar sem senha ou usar o
  .p12), em vez de suportar o formato legado de PEM cifrado.

## Consequências

- ➕ Certificado errado aparece no cadastro, com o motivo, e não na primeira cobrança.
- ➕ Alerta de vencimento (#72) e regra de ambiente (#73) usam os dados públicos sem decifrar nada.
- ➕ A impressão digital deixa conferir, sem expor o certificado, qual arquivo está em cada conta.
- ➖ Uma dependência externa a mais no caminho de segredo. É pequena, só depende de `golang.org/x/crypto` e é auditável.
- ➖ Sujeito e validade ficam em claro no banco. São dados públicos de qualquer certificado, mas dizem com qual banco a instância trabalha.

[lib]: https://pkg.go.dev/software.sslmate.com/src/go-pkcs12
