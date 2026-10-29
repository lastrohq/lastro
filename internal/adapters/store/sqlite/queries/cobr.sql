-- name: CriarCobr :exec
INSERT INTO cobrancas_recorrentes (txid, recorrencia_id, conta_id, status, valor, vencimento, ajuste_dia_util, info_adicional,
                                   tentativas, motivo_encerramento, criada_em, revisao)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: BuscarCobr :one
SELECT txid, recorrencia_id, conta_id, status, valor, vencimento, ajuste_dia_util, info_adicional,
       tentativas, motivo_encerramento, criada_em, revisao
FROM cobrancas_recorrentes WHERE txid = ?;

-- name: AtualizarCobr :execrows
UPDATE cobrancas_recorrentes SET status = ?, tentativas = ?, motivo_encerramento = ?, revisao = revisao + 1
WHERE txid = ? AND revisao = ?;

-- name: ExisteCobr :one
SELECT COUNT(*) FROM cobrancas_recorrentes WHERE txid = ?;

-- name: CobrsDaRecorrencia :many
SELECT txid, recorrencia_id, conta_id, status, valor, vencimento, ajuste_dia_util, info_adicional,
       tentativas, motivo_encerramento, criada_em, revisao
FROM cobrancas_recorrentes WHERE recorrencia_id = ? ORDER BY vencimento, criada_em, txid;
