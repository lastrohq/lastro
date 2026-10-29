-- name: AdicionarEvento :exec
INSERT INTO outbox (id, tipo, conta_id, ocorrido_em, dados) VALUES (?, ?, ?, ?, ?);

-- name: NaoDistribuidos :many
SELECT id, tipo, conta_id, ocorrido_em, dados FROM outbox WHERE distribuido = 0 ORDER BY seq LIMIT ?;

-- name: MarcarDistribuido :exec
UPDATE outbox SET distribuido = 1 WHERE id = ?;
