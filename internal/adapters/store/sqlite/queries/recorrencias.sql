-- name: CriarRecorrencia :exec
INSERT INTO recorrencias (id, conta_id, status, contrato, objeto, devedor_documento, devedor_nome, data_inicial, data_final,
                          periodicidade, valor_fixo, piso_valor_maximo, politica, motivo_rejeicao, criada_em, revisao)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: BuscarRecorrencia :one
SELECT id, conta_id, status, contrato, objeto, devedor_documento, devedor_nome, data_inicial, data_final,
       periodicidade, valor_fixo, piso_valor_maximo, politica, motivo_rejeicao, criada_em, revisao
FROM recorrencias WHERE id = ?;

-- name: AtualizarRecorrencia :execrows
UPDATE recorrencias SET status = ?, motivo_rejeicao = ?, revisao = revisao + 1 WHERE id = ? AND revisao = ?;

-- name: ExisteRecorrencia :one
SELECT COUNT(*) FROM recorrencias WHERE id = ?;

-- name: CriarSolicitacao :exec
INSERT INTO solicitacoes_recorrencia (id, recorrencia_id, conta_id, status, ispb, agencia, conta, expira_em, criada_em)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: BuscarSolicitacao :one
SELECT id, recorrencia_id, conta_id, status, ispb, agencia, conta, expira_em, criada_em
FROM solicitacoes_recorrencia WHERE id = ?;

-- name: AtualizarStatusSolicitacao :execrows
UPDATE solicitacoes_recorrencia SET status = ? WHERE id = ?;

-- name: SolicitacoesDaRecorrencia :many
SELECT id, recorrencia_id, conta_id, status, ispb, agencia, conta, expira_em, criada_em
FROM solicitacoes_recorrencia WHERE recorrencia_id = ? ORDER BY criada_em, id;

-- name: RecorrenciasAbertas :many
SELECT id FROM recorrencias WHERE conta_id = ? AND status IN ('CRIADA', 'APROVADA') ORDER BY criada_em, id;
