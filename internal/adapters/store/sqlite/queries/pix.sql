-- name: RegistrarPix :execrows
INSERT INTO pix_recebidos (end_to_end_id, conta_id, txid, valor, chave, horario, info_pagador, pagador_documento, pagador_nome)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (end_to_end_id) DO NOTHING;

-- name: BuscarPixPorE2E :one
SELECT * FROM pix_recebidos WHERE end_to_end_id = ?;

-- name: ListarPixPorTxid :many
SELECT * FROM pix_recebidos WHERE txid = ? ORDER BY horario, end_to_end_id;
