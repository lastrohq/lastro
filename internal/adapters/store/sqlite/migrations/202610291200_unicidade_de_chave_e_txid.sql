-- +goose Up
-- Chave Pix pertence a uma conta só (como no DICT) e txid é único na instância (RN-10).
CREATE UNIQUE INDEX contas_chave_pix ON contas (chave_pix);
CREATE UNIQUE INDEX cobrancas_txid ON cobrancas (txid);
CREATE INDEX cobrancas_por_documento ON cobrancas (devedor_documento);

-- +goose Down
DROP INDEX cobrancas_por_documento;
DROP INDEX cobrancas_txid;
DROP INDEX contas_chave_pix;
