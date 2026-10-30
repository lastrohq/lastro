// Package sicoob implementa o port PSP para o Sicoob, pela API Pix
// Recebimentos (https://developers.sicoob.com.br). O comportamento comum da
// spec do BCB vem do pixapi; aqui ficam a configuração e os desvios.
//
// Autenticação: mTLS com o certificado e-CNPJ cadastrado na aplicação do
// portal Sicoob Developers, em toda chamada, inclusive a do token. O token é
// OAuth2 client credentials no Keycloak do Sicoob (auth.sicoob.com.br, realm
// cooperado) e leva só o client_id e o scope: não há client secret, quem
// identifica a aplicação é o certificado. O token vale poucos minutos; o
// cache do pixapi reusa o mesmo até perto de expirar.
//
// Fontes: o portal do Sicoob só mostra o swagger com login, então o adapter
// segue a spec do BCB e o que o projeto kmee/brbank registrou ao rodar contra o
// sandbox do Sicoob em 20/10/2026 (docs/pix-duvidas.md de lá). Nada disso foi
// conferido ainda com uma conta de cooperado.
//
// Desvios do Sicoob em relação à spec do BCB, tratados aqui (RN-51):
//
//   - Toda chamada à API Pix leva o header client_id, além do token.
//   - A cob e a cobv devolvem o BR Code no campo brcode, não no
//     pixCopiaECola (o pixapi lê os dois).
//   - A cob só aceita documento e nome no devedor; endereço e e-mail são
//     recusados com 400 (é o schema da spec, que o Lastro já segue).
//   - Não há documentação pública do que acontece com txid nem com id de
//     devolução repetidos: o adapter consulta antes de criar a cob, a cobv e a
//     devolução, e cumpre o port (ErrJaExiste, idempotência) sem depender disso.
//   - O sandbox (sandbox.sicoob.com.br) não usa mTLS nem OAuth (token fixo do
//     portal), valida só o formato do pedido e responde dados aleatórios, sem
//     guardar estado. Não serve de homologação para o Lastro: a conta do Sicoob
//     é sempre de produção, e para testar sem banco existe o sandbox do Lastro.
//
// Erros vêm no formato de problema da spec (RFC 7807) ou, quando o gateway do
// Sicoob recusa antes da API, como {"httpCode", "httpMessage",
// "moreInformation"}. NaoEncontrado (ou 404) vira ErrNaoEncontrado; o resto
// vira ErrSicoob com o que o banco disse.
package sicoob
