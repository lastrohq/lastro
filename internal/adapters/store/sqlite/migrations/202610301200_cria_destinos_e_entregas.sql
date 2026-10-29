-- +goose Up
-- Evento do outbox já distribuído em entregas (ADR-005).
ALTER TABLE outbox ADD COLUMN distribuido INTEGER NOT NULL DEFAULT 0;
CREATE INDEX outbox_nao_distribuidos ON outbox (distribuido, seq);

-- Destinos de webhook de saída. conta_id NULL = todas as contas.
CREATE TABLE destinos_webhook (
    id              TEXT    PRIMARY KEY,
    conta_id        TEXT    REFERENCES contas (id),
    url             TEXT    NOT NULL,
    segredo_cifrado BLOB    NOT NULL,
    criado_em       INTEGER NOT NULL
) STRICT;

-- Uma entrega por (evento, destino). corpo é montado na distribuição e repetido
-- byte a byte em cada tentativa. criada_em é quando o evento aconteceu.
CREATE TABLE entregas_webhook (
    evento_id     TEXT    NOT NULL,
    destino_id    TEXT    NOT NULL,
    conta_id      TEXT    NOT NULL,
    tipo          TEXT    NOT NULL,
    corpo         BLOB    NOT NULL,
    criada_em     INTEGER NOT NULL,
    status        TEXT    NOT NULL CHECK (status IN ('pendente', 'entregue', 'dlq')),
    tentativas    INTEGER NOT NULL,
    proxima_em    INTEGER NOT NULL,
    ultimo_http   INTEGER NOT NULL,
    ultimo_erro   TEXT    NOT NULL,
    atualizada_em INTEGER NOT NULL,
    PRIMARY KEY (evento_id, destino_id)
) STRICT;

CREATE INDEX entregas_fila ON entregas_webhook (status, proxima_em);

-- +goose Down
DROP TABLE entregas_webhook;
DROP TABLE destinos_webhook;
DROP INDEX outbox_nao_distribuidos;
ALTER TABLE outbox DROP COLUMN distribuido;
