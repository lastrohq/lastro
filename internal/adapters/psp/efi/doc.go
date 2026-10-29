// Package efi implementa o port PSP para o Efí (antiga Gerencianet), pela API
// Pix: https://dev.efipay.com.br/docs/api-pix/.
//
// Autenticação: mTLS com o certificado da conta em toda chamada, inclusive a do
// token, e OAuth2 client credentials (Basic auth com client id e secret, corpo
// JSON). O token é reutilizado até perto de expirar (internal/platform/oauth).
//
// Desvios do Efí em relação à spec do BCB, tratados aqui (RN-51):
//
//   - Erro vem como {"nome": ..., "mensagem": ...}, não no formato de problema
//     da spec. Os nomes conhecidos viram os erros do port (txid_duplicado →
//     ErrJaExiste, cobranca_nao_encontrada → ErrNaoEncontrado,
//     status_cobranca_invalido → ErrTransicaoInvalida).
//   - calendario.expiracao é número no envio e às vezes string ("3600") na
//     resposta do GET.
//   - Na cobv, o erro vem no formato de problema da spec (title/detail), e o
//     Efí não documenta um nome para txid repetido. Se o PUT falha com 4xx e
//     o txid já existe, o erro vira ErrJaExiste. Isso precisa ser confirmado
//     na homologação.
//   - Devolução com id repetido responde 409 devolucao_id_duplicado. O port
//     pede idempotência, então o adapter devolve a devolução existente se o
//     valor for o mesmo, e ErrJaExiste se for outro.
//   - GET /v2/pix filtra com fim inclusivo; o port pede [Inicio, Fim), e o
//     adapter descarta o que cai exatamente em Fim.
//
// Homologação: só cobranças de R$ 0,01 a R$ 10,00 são confirmadas e geram
// webhook; acima disso, a cobrança fica ATIVA.
package efi
