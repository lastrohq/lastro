-- name: CriarCobranca :exec
INSERT INTO cobrancas (
    conta_id, txid, chave, status, valor, permite_alteracao, expiracao_s, devedor_documento, devedor_nome,
    solicitacao_pagador, location, br_code, criada_em, revisao
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0);

-- name: BuscarCobranca :one
SELECT * FROM cobrancas WHERE conta_id = ? AND txid = ?;

-- name: BuscarCobrancaPorTxid :one
SELECT * FROM cobrancas WHERE txid = ?;

-- name: AtualizarCobranca :execrows
UPDATE cobrancas SET
    status = @status, valor = @valor, permite_alteracao = @permite_alteracao, expiracao_s = @expiracao_s,
    devedor_documento = @devedor_documento, devedor_nome = @devedor_nome,
    solicitacao_pagador = @solicitacao_pagador, location = @location, br_code = @br_code,
    revisao = revisao + 1
WHERE conta_id = @conta_id AND txid = @txid AND revisao = @revisao;

-- name: ListarCobrancas :many
SELECT * FROM cobrancas
WHERE seq > @depois_de
  AND (sqlc.narg(conta_id) IS NULL OR conta_id = sqlc.narg(conta_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(documento) IS NULL OR devedor_documento = sqlc.narg(documento))
  AND (sqlc.narg(inicio) IS NULL OR criada_em >= sqlc.narg(inicio))
  AND (sqlc.narg(fim) IS NULL OR criada_em < sqlc.narg(fim))
ORDER BY seq
LIMIT @limite OFFSET @pular;

-- name: ContarCobrancas :one
SELECT COUNT(*) FROM cobrancas
WHERE (sqlc.narg(conta_id) IS NULL OR conta_id = sqlc.narg(conta_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(documento) IS NULL OR devedor_documento = sqlc.narg(documento))
  AND (sqlc.narg(inicio) IS NULL OR criada_em >= sqlc.narg(inicio))
  AND (sqlc.narg(fim) IS NULL OR criada_em < sqlc.narg(fim));
