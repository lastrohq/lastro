-- +goose Up
-- Devoluções de Pix recebidos (RN-20, RN-21). O id é do cliente e único por Pix.
CREATE TABLE devolucoes (
    end_to_end_id TEXT    NOT NULL REFERENCES pix_recebidos (end_to_end_id),
    id            TEXT    NOT NULL,
    conta_id      TEXT    NOT NULL REFERENCES contas (id),
    valor         INTEGER NOT NULL CHECK (valor > 0),
    natureza      TEXT    NOT NULL,
    descricao     TEXT    NOT NULL,
    status        TEXT    NOT NULL,
    motivo        TEXT    NOT NULL,
    rtr_id        TEXT    NOT NULL,
    criada_em     INTEGER NOT NULL,
    liquidada_em  INTEGER,
    revisao       INTEGER NOT NULL,
    PRIMARY KEY (end_to_end_id, id)
) STRICT;

CREATE INDEX devolucoes_em_processamento ON devolucoes (conta_id, criada_em) WHERE status = 'EM_PROCESSAMENTO';

-- +goose Down
DROP TABLE devolucoes;
