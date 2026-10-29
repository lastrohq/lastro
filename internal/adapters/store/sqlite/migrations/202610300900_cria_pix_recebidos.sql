-- +goose Up
-- Um Pix por endToEndId: webhook repetido e conciliação não duplicam (RN-30, RN-31).
CREATE TABLE pix_recebidos (
    end_to_end_id    TEXT    PRIMARY KEY,
    conta_id         TEXT    NOT NULL REFERENCES contas (id),
    txid             TEXT    NOT NULL,
    valor            INTEGER NOT NULL CHECK (valor > 0),
    chave            TEXT    NOT NULL,
    horario          INTEGER NOT NULL,
    info_pagador     TEXT    NOT NULL,
    pagador_documento TEXT,
    pagador_nome     TEXT
) STRICT;

CREATE INDEX pix_por_txid ON pix_recebidos (txid, horario);
CREATE INDEX pix_por_conta_horario ON pix_recebidos (conta_id, horario);

-- Hash do token da URL de webhook da conta.
ALTER TABLE contas ADD COLUMN webhook_token_hash BLOB;

-- +goose Down
ALTER TABLE contas DROP COLUMN webhook_token_hash;
DROP TABLE pix_recebidos;
