-- +goose Up
-- Cobrança recorrente do Pix Automático (RN-42). O txid continua único na
-- instância (RN-10): a tabela txids passa a aceitar 'cobr'. O SQLite não altera
-- CHECK, então txids é refeita, e os gatilhos que a usam, recriados.
DROP TRIGGER cobrancas_registra_txid;
DROP TRIGGER cobv_registra_txid;

CREATE TABLE txids_novo (
    txid TEXT PRIMARY KEY,
    tipo TEXT NOT NULL CHECK (tipo IN ('cob', 'cobv', 'cobr'))
) STRICT;
INSERT INTO txids_novo (txid, tipo) SELECT txid, tipo FROM txids;
DROP TABLE txids;
ALTER TABLE txids_novo RENAME TO txids;

-- +goose StatementBegin
CREATE TRIGGER cobrancas_registra_txid AFTER INSERT ON cobrancas
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cob');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER cobv_registra_txid AFTER INSERT ON cobrancas_vencimento
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cobv');
END;
-- +goose StatementEnd

-- As tentativas ficam em JSON: são lidas junto com a cobrança e nunca filtradas.
CREATE TABLE cobrancas_recorrentes (
    txid                TEXT    PRIMARY KEY,
    recorrencia_id      TEXT    NOT NULL REFERENCES recorrencias (id),
    conta_id            TEXT    NOT NULL REFERENCES contas (id),
    status              TEXT    NOT NULL,
    valor               INTEGER NOT NULL CHECK (valor > 0),
    vencimento          TEXT    NOT NULL,
    ajuste_dia_util     INTEGER NOT NULL CHECK (ajuste_dia_util IN (0, 1)),
    info_adicional      TEXT    NOT NULL,
    tentativas          TEXT    NOT NULL,
    motivo_encerramento TEXT    NOT NULL,
    criada_em           INTEGER NOT NULL,
    revisao             INTEGER NOT NULL
) STRICT;

CREATE INDEX cobr_por_recorrencia ON cobrancas_recorrentes (recorrencia_id, vencimento);

-- +goose StatementBegin
CREATE TRIGGER cobr_registra_txid AFTER INSERT ON cobrancas_recorrentes
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cobr');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER cobr_registra_txid;
DROP TABLE cobrancas_recorrentes;
DROP TRIGGER cobrancas_registra_txid;
DROP TRIGGER cobv_registra_txid;

CREATE TABLE txids_antigo (
    txid TEXT PRIMARY KEY,
    tipo TEXT NOT NULL CHECK (tipo IN ('cob', 'cobv'))
) STRICT;
INSERT INTO txids_antigo (txid, tipo) SELECT txid, tipo FROM txids WHERE tipo <> 'cobr';
DROP TABLE txids;
ALTER TABLE txids_antigo RENAME TO txids;

-- +goose StatementBegin
CREATE TRIGGER cobrancas_registra_txid AFTER INSERT ON cobrancas
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cob');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER cobv_registra_txid AFTER INSERT ON cobrancas_vencimento
BEGIN
    INSERT INTO txids (txid, tipo) VALUES (NEW.txid, 'cobv');
END;
-- +goose StatementEnd
