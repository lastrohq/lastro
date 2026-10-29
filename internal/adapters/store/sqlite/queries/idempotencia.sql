-- name: BuscarIdempotencia :one
SELECT * FROM idempotencia WHERE dono = ? AND chave = ?;

-- name: SalvarIdempotencia :exec
INSERT INTO idempotencia (dono, chave, hash, status, content_type, corpo, criada_em)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (dono, chave) DO UPDATE SET
    hash = excluded.hash, status = excluded.status, content_type = excluded.content_type,
    corpo = excluded.corpo, criada_em = excluded.criada_em;

-- name: RemoverIdempotencia :exec
DELETE FROM idempotencia WHERE dono = ? AND chave = ?;
