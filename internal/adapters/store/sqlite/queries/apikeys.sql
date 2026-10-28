-- name: CriarAPIKey :exec
INSERT INTO api_keys (id, nome, hash, escopos, conta_id, criada_em) VALUES (?, ?, ?, ?, ?, ?);

-- name: BuscarAPIKey :one
SELECT * FROM api_keys WHERE id = ?;

-- name: ListarAPIKeys :many
SELECT * FROM api_keys ORDER BY criada_em, id;

-- name: RevogarAPIKey :execrows
UPDATE api_keys SET revogada_em = COALESCE(revogada_em, @quando) WHERE id = @id;
