-- +goose Up
-- Respostas guardadas por 24 h para o Idempotency-Key (RN-11). status 0 = em andamento.
CREATE TABLE idempotencia (
    dono         TEXT    NOT NULL,
    chave        TEXT    NOT NULL,
    hash         BLOB    NOT NULL,
    status       INTEGER NOT NULL,
    content_type TEXT    NOT NULL,
    corpo        BLOB    NOT NULL,
    criada_em    INTEGER NOT NULL,
    PRIMARY KEY (dono, chave)
) STRICT;

CREATE INDEX idempotencia_por_criacao ON idempotencia (criada_em);

-- +goose Down
DROP TABLE idempotencia;
