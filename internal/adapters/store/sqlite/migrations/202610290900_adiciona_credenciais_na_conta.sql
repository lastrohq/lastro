-- +goose Up
-- Blob cifrado pelo Vault (AES-256-GCM). O banco nunca vê o segredo em claro (RN-61).
ALTER TABLE contas ADD COLUMN credenciais BLOB;

-- +goose Down
ALTER TABLE contas DROP COLUMN credenciais;
