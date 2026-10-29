-- +goose Up
-- O txid é único na instância, entre cob e cobv (RN-10). Os gatilhos registram
-- todo txid aqui: a chave primária recusa o repetido, venha da tabela que vier.
CREATE TABLE txids (
    txid TEXT PRIMARY KEY,
    tipo TEXT NOT NULL CHECK (tipo IN ('cob', 'cobv'))
) STRICT;

INSERT INTO txids (txid, tipo) SELECT txid, 'cob' FROM cobrancas;

-- +goose StatementBegin
CREATE TRIGGER cobrancas_registra_txid AFTER INSERT ON cobrancas
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cob');
END;
-- +goose StatementEnd

-- Multa, juros, abatimento, desconto e os dados opcionais do devedor ficam em
-- detalhes (JSON): são lidos junto com a cobrança e nunca filtrados.
CREATE TABLE cobrancas_vencimento (
    seq                 INTEGER PRIMARY KEY AUTOINCREMENT,
    conta_id            TEXT    NOT NULL REFERENCES contas (id),
    txid                TEXT    NOT NULL UNIQUE,
    chave               TEXT    NOT NULL,
    status              TEXT    NOT NULL,
    valor               INTEGER NOT NULL CHECK (valor > 0),
    vencimento          TEXT    NOT NULL,
    validade_dias       INTEGER NOT NULL CHECK (validade_dias >= 0),
    devedor_documento   TEXT    NOT NULL,
    devedor_nome        TEXT    NOT NULL,
    detalhes            TEXT    NOT NULL,
    solicitacao_pagador TEXT    NOT NULL,
    location            TEXT    NOT NULL,
    br_code             TEXT    NOT NULL,
    criada_em           INTEGER NOT NULL,
    revisao             INTEGER NOT NULL
) STRICT;

CREATE INDEX cobv_por_conta_status ON cobrancas_vencimento (conta_id, status, seq);
CREATE INDEX cobv_por_criacao ON cobrancas_vencimento (criada_em);
CREATE INDEX cobv_por_documento ON cobrancas_vencimento (devedor_documento);

-- +goose StatementBegin
CREATE TRIGGER cobv_registra_txid AFTER INSERT ON cobrancas_vencimento
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cobv');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER cobv_registra_txid;
DROP TABLE cobrancas_vencimento;
DROP TRIGGER cobrancas_registra_txid;
DROP TABLE txids;
