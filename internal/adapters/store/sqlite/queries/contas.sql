-- name: CriarConta :exec
INSERT INTO contas (id, nome, psp, chave_pix, ambiente, ativa) VALUES (?, ?, ?, ?, ?, ?);

-- name: BuscarConta :one
SELECT id, nome, psp, chave_pix, ambiente, ativa FROM contas WHERE id = ?;

-- name: ListarContas :many
SELECT id, nome, psp, chave_pix, ambiente, ativa FROM contas ORDER BY id;

-- name: AtualizarConta :execrows
UPDATE contas SET nome = ?, chave_pix = ?, ambiente = ?, ativa = ? WHERE id = ?;

-- name: SalvarCredenciais :execrows
UPDATE contas SET credenciais = ? WHERE id = ?;

-- name: BuscarCredenciais :one
SELECT credenciais FROM contas WHERE id = ?;

-- name: BuscarContaPorChave :one
SELECT id, nome, psp, chave_pix, ambiente, ativa FROM contas WHERE chave_pix = ?;

-- name: SalvarTokenWebhook :execrows
UPDATE contas SET webhook_token_hash = ? WHERE id = ?;

-- name: BuscarTokenWebhook :one
SELECT webhook_token_hash FROM contas WHERE id = ?;
