-- +goose Up
-- A chave da cobrança é a da conta no momento da criação: se a conta trocar de
-- chave depois, as cobranças antigas continuam mostrando a chave com que nasceram.
ALTER TABLE cobrancas ADD COLUMN chave TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE cobrancas DROP COLUMN chave;
