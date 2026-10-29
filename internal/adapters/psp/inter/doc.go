// Package inter implementa o port PSP para o Banco Inter (Inter Empresas),
// pela API Pix: https://developers.inter.co/references/pix. O comportamento
// comum da spec do BCB vem do pixapi; aqui ficam a configuração e os desvios.
//
// Autenticação: mTLS com o certificado da integração (o Inter entrega .crt e
// .key; junte os dois num PEM para cadastrar a conta) em toda chamada,
// inclusive a do token, e OAuth2 client credentials com client_id,
// client_secret e scope no corpo do formulário, em /oauth/v2/token. O token
// vale uma hora e o Inter limita a 5 emissões por minuto: o cache do pixapi
// reusa o mesmo token até perto de expirar.
//
// Desvios do Inter em relação à spec do BCB, tratados aqui (RN-51), segundo o
// swagger-api-pix.yaml oficial (consultado em 29/10/2026; ainda não conferidos
// na homologação):
//
//   - O PUT /cob/:txid só documenta 400 genérico (CobOperacaoInvalida), sem
//     erro próprio para txid repetido, e nada diz sobre o PUT /cobv repetido
//     nem sobre id de devolução repetido. O adapter consulta antes de criar a
//     cob, a cobv e a devolução, e cumpre o port (ErrJaExiste, idempotência)
//     sem depender disso.
//   - GET /pix usa o filtro txId (com I maiúsculo) em vez de txid; o adapter
//     não usa esse filtro.
//   - A simulação de pagamento do sandbox (POST /sandbox/cob/pagamento) recebe
//     o valor como número em reais, não como a string da spec.
//   - Aplicação ligada a mais de uma conta corrente exige o cabeçalho
//     x-conta-corrente; o adapter ainda não envia, então use uma aplicação
//     por conta.
//
// Erros vêm no formato de problema da spec (RFC 7807): NaoEncontrado (ou 404)
// vira ErrNaoEncontrado; o resto vira ErrInter com o tipo e o detalhe.
package inter
