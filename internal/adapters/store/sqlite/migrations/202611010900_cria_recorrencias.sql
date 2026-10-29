-- +goose Up
-- Pix Automático (RN-40, RN-41). O idRec e o idSolicRec vêm do PSP.
-- Datas civis em texto AAAA-MM-DD; instantes em nanossegundos Unix UTC; valores em centavos.
CREATE TABLE recorrencias (
    id                TEXT    PRIMARY KEY,
    conta_id          TEXT    NOT NULL REFERENCES contas (id),
    status            TEXT    NOT NULL,
    contrato          TEXT    NOT NULL,
    objeto            TEXT    NOT NULL,
    devedor_documento TEXT    NOT NULL,
    devedor_nome      TEXT    NOT NULL,
    data_inicial      TEXT    NOT NULL,
    data_final        TEXT,
    periodicidade     TEXT    NOT NULL,
    valor_fixo        INTEGER NOT NULL CHECK (valor_fixo >= 0),
    piso_valor_maximo INTEGER NOT NULL CHECK (piso_valor_maximo >= 0),
    politica          TEXT    NOT NULL,
    motivo_rejeicao   TEXT    NOT NULL,
    criada_em         INTEGER NOT NULL,
    revisao           INTEGER NOT NULL
) STRICT;

CREATE TABLE solicitacoes_recorrencia (
    id             TEXT    PRIMARY KEY,
    recorrencia_id TEXT    NOT NULL REFERENCES recorrencias (id),
    conta_id       TEXT    NOT NULL REFERENCES contas (id),
    status         TEXT    NOT NULL,
    ispb           TEXT    NOT NULL,
    agencia        TEXT    NOT NULL,
    conta          TEXT    NOT NULL,
    expira_em      INTEGER NOT NULL,
    criada_em      INTEGER NOT NULL
) STRICT;

CREATE INDEX solicitacoes_por_recorrencia ON solicitacoes_recorrencia (recorrencia_id);

-- +goose Down
DROP TABLE solicitacoes_recorrencia;
DROP TABLE recorrencias;
