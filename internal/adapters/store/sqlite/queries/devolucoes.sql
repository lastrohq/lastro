-- name: CriarDevolucao :exec
INSERT INTO devolucoes (end_to_end_id, id, conta_id, valor, natureza, descricao, status, motivo, rtr_id, criada_em, liquidada_em, revisao)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: BuscarDevolucao :one
SELECT * FROM devolucoes WHERE end_to_end_id = ? AND id = ?;

-- name: DevolucoesDoPix :many
SELECT * FROM devolucoes WHERE end_to_end_id = ? ORDER BY criada_em, id;

-- name: AtualizarDevolucao :execrows
UPDATE devolucoes SET status = ?, motivo = ?, rtr_id = ?, liquidada_em = ?, revisao = revisao + 1
WHERE end_to_end_id = ? AND id = ? AND revisao = ?;

-- name: DevolucoesEmProcessamento :many
SELECT * FROM devolucoes WHERE conta_id = ? AND status = 'EM_PROCESSAMENTO' ORDER BY criada_em, end_to_end_id, id;
