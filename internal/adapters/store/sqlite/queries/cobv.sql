-- name: CriarCobv :exec
INSERT INTO cobrancas_vencimento (
    conta_id, txid, chave, status, valor, vencimento, validade_dias, devedor_documento, devedor_nome,
    detalhes, solicitacao_pagador, location, br_code, criada_em, revisao
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0);

-- name: BuscarCobvPorTxid :one
SELECT * FROM cobrancas_vencimento WHERE txid = ?;

-- name: AtualizarCobv :execrows
UPDATE cobrancas_vencimento SET
    status = @status, valor = @valor, vencimento = @vencimento, validade_dias = @validade_dias,
    devedor_documento = @devedor_documento, devedor_nome = @devedor_nome, detalhes = @detalhes,
    solicitacao_pagador = @solicitacao_pagador, location = @location, br_code = @br_code,
    revisao = revisao + 1
WHERE txid = @txid AND revisao = @revisao;

-- name: ListarCobvs :many
SELECT * FROM cobrancas_vencimento
WHERE seq > @depois_de
  AND (sqlc.narg(conta_id) IS NULL OR conta_id = sqlc.narg(conta_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(documento) IS NULL OR devedor_documento = sqlc.narg(documento))
  AND (sqlc.narg(inicio) IS NULL OR criada_em >= sqlc.narg(inicio))
  AND (sqlc.narg(fim) IS NULL OR criada_em < sqlc.narg(fim))
ORDER BY seq
LIMIT @limite OFFSET @pular;

-- name: ContarCobvs :one
SELECT COUNT(*) FROM cobrancas_vencimento
WHERE (sqlc.narg(conta_id) IS NULL OR conta_id = sqlc.narg(conta_id))
  AND (sqlc.narg(status) IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(documento) IS NULL OR devedor_documento = sqlc.narg(documento))
  AND (sqlc.narg(inicio) IS NULL OR criada_em >= sqlc.narg(inicio))
  AND (sqlc.narg(fim) IS NULL OR criada_em < sqlc.narg(fim));
