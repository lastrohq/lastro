-- +goose Up
-- Só o hash SHA-256 do segredo é gravado (RN-60). escopos é separado por espaço.
CREATE TABLE api_keys (
    id          TEXT    PRIMARY KEY,
    nome        TEXT    NOT NULL,
    hash        BLOB    NOT NULL,
    escopos     TEXT    NOT NULL,
    conta_id    TEXT    REFERENCES contas (id),
    criada_em   INTEGER NOT NULL,
    revogada_em INTEGER
) STRICT;

-- +goose Down
DROP TABLE api_keys;
