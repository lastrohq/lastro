-- +goose Up
CREATE TABLE contas (
    id        TEXT    PRIMARY KEY,
    nome      TEXT    NOT NULL,
    psp       TEXT    NOT NULL,
    chave_pix TEXT    NOT NULL,
    ambiente  TEXT    NOT NULL CHECK (ambiente IN ('homologacao', 'producao')),
    ativa     INTEGER NOT NULL
) STRICT;

-- Dinheiro em centavos (RN-02), instantes em nanossegundos Unix UTC.
-- seq dá a ordem de criação e é o cursor da paginação.
CREATE TABLE cobrancas (
    seq                 INTEGER PRIMARY KEY AUTOINCREMENT,
    conta_id            TEXT    NOT NULL REFERENCES contas (id),
    txid                TEXT    NOT NULL,
    status              TEXT    NOT NULL,
    valor               INTEGER NOT NULL CHECK (valor >= 0),
    permite_alteracao   INTEGER NOT NULL,
    expiracao_s         INTEGER NOT NULL,
    devedor_documento   TEXT,
    devedor_nome        TEXT,
    solicitacao_pagador TEXT    NOT NULL,
    location            TEXT    NOT NULL,
    br_code             TEXT    NOT NULL,
    criada_em           INTEGER NOT NULL,
    revisao             INTEGER NOT NULL,
    UNIQUE (conta_id, txid)
) STRICT;

CREATE INDEX cobrancas_por_conta_status ON cobrancas (conta_id, status, seq);
CREATE INDEX cobrancas_por_criacao ON cobrancas (criada_em);

CREATE TABLE outbox (
    seq         INTEGER PRIMARY KEY AUTOINCREMENT,
    id          TEXT    NOT NULL UNIQUE,
    tipo        TEXT    NOT NULL,
    conta_id    TEXT    NOT NULL,
    ocorrido_em INTEGER NOT NULL,
    dados       TEXT    NOT NULL
) STRICT;

-- +goose Down
DROP TABLE outbox;
DROP TABLE cobrancas;
DROP TABLE contas;
