-- name: AdicionarEvento :exec
INSERT INTO outbox (id, tipo, conta_id, ocorrido_em, dados) VALUES (?, ?, ?, ?, ?);
