// Package bb implementa o port PSP para o Banco do Brasil, pela API Pix v2 do
// Portal Developers BB (https://developers.bb.com.br). O comportamento comum
// da spec do BCB vem do pixapi; aqui ficam a configuração e os desvios.
//
// Autenticação: OAuth2 client credentials com client_id e client_secret em
// Basic, e scope obrigatório, em oauth.bb.com.br (oauth.hm.bb.com.br na
// homologação). Toda chamada à API leva a chave de aplicação do portal
// (developer_application_key) na query: gw-dev-app-key na homologação,
// gw-app-key em produção. Em produção (api-pix.bb.com.br) a API exige mTLS
// com um certificado A1 ICP-Brasil (de preferência e-CNPJ) cadastrado na
// aplicação; a homologação em api.hm.bb.com.br não pede certificado.
//
// Fontes: o portal do BB, o que o projeto kmee/brbank registrou ao rodar cob
// e cobv contra a homologação do BB em 28/10/2026 (docs/bb-homologacao.md de
// lá) e SDKs de terceiros para o nome da chave em produção. Nada disso foi
// conferido ainda pelo Lastro.
//
// Desvios do BB em relação à spec do BCB, tratados aqui (RN-51):
//
//   - A chave de aplicação vai na query de toda chamada, com nome diferente
//     por ambiente.
//   - A cob e a cobv devolvem o BR Code em textoImagemQRcode, não em
//     pixCopiaECola (o pixapi lê os dois).
//   - Não há documentação pública do que acontece com txid nem com id de
//     devolução repetidos, e a homologação é compartilhada por todas as
//     aplicações: o adapter consulta antes de criar a cob, a cobv e a
//     devolução, e cumpre o port (ErrJaExiste, idempotência) sem depender
//     disso.
//   - A homologação não dispara webhook. Ela tem um simulador de pagamento
//     (POST /testes-portal-desenvolvedor/v1/boletos-pix/pagar, com o BR Code)
//     que conclui a cobrança em até uns 20 segundos: SimularPagamento.
//
// Erros vêm no formato de problema da spec (RFC 7807) ou como lista de erros
// do gateway do BB (erros/errors, com mensagem/message). NaoEncontrado (ou
// 404) vira ErrNaoEncontrado; o resto vira ErrBB com o que o banco disse.
package bb
