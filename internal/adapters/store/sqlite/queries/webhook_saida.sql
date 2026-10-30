-- name: CriarDestino :exec
INSERT INTO destinos_webhook (id, conta_id, url, segredo_cifrado, criado_em) VALUES (?, ?, ?, ?, ?);

-- name: ListarDestinos :many
SELECT * FROM destinos_webhook ORDER BY criado_em, id;

-- name: DestinosDaConta :many
SELECT * FROM destinos_webhook WHERE conta_id IS NULL OR conta_id = ? ORDER BY criado_em, id;

-- name: AtualizarSegredoDestino :execrows
UPDATE destinos_webhook SET segredo_cifrado = ? WHERE id = ?;

-- name: RemoverDestino :execrows
DELETE FROM destinos_webhook WHERE id = ?;

-- name: CriarEntrega :exec
INSERT INTO entregas_webhook (evento_id, destino_id, conta_id, tipo, corpo, criada_em, status, tentativas, proxima_em, ultimo_http, ultimo_erro, atualizada_em)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (evento_id, destino_id) DO NOTHING;

-- name: ProximasEntregas :many
SELECT * FROM entregas_webhook WHERE status = 'pendente' AND proxima_em <= ? ORDER BY proxima_em, evento_id, destino_id LIMIT ?;

-- name: AtualizarEntrega :execrows
UPDATE entregas_webhook SET status = ?, tentativas = ?, proxima_em = ?, ultimo_http = ?, ultimo_erro = ?, atualizada_em = ?
WHERE evento_id = ? AND destino_id = ?;

-- name: ListarEntregas :many
SELECT * FROM entregas_webhook WHERE status = ? ORDER BY proxima_em, evento_id, destino_id LIMIT ?;

-- name: ReenviarEntregas :execrows
UPDATE entregas_webhook SET status = 'pendente', tentativas = 0, proxima_em = @agora, atualizada_em = @agora
WHERE evento_id = @evento_id AND status = 'dlq';
