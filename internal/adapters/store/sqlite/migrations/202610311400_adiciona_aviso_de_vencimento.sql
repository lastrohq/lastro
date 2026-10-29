-- +goose Up
-- Impressão digital do certificado cujo vencimento já foi avisado (RN-62). Guarda a
-- impressão, e não um booleano, para que trocar o certificado zere o aviso sozinho.
ALTER TABLE contas ADD COLUMN cert_avisado_impressao TEXT;

-- +goose Down
ALTER TABLE contas DROP COLUMN cert_avisado_impressao;
