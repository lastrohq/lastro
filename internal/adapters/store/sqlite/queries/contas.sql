-- name: CriarConta :exec
INSERT INTO contas (id, nome, psp, chave_pix, ambiente, ativa) VALUES (?, ?, ?, ?, ?, ?);

-- name: BuscarConta :one
SELECT * FROM contas WHERE id = ?;

-- name: ListarContas :many
SELECT * FROM contas ORDER BY id;
