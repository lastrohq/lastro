-- +goose Up
-- Dados públicos do certificado do mTLS, fora do blob cifrado, para o alerta de
-- vencimento e a regra de ambiente (RN-62). O certificado e a chave continuam no blob.
-- A validade vai em segundos Unix, não em nanossegundos como os outros instantes:
-- certificado pode valer até 9999-12-31 (RFC 5280), além do que cabe em int64 de ns.
ALTER TABLE contas ADD COLUMN cert_sujeito TEXT;
ALTER TABLE contas ADD COLUMN cert_impressao_digital TEXT;
ALTER TABLE contas ADD COLUMN cert_validade_s INTEGER;

-- +goose Down
ALTER TABLE contas DROP COLUMN cert_validade_s;
ALTER TABLE contas DROP COLUMN cert_impressao_digital;
ALTER TABLE contas DROP COLUMN cert_sujeito;
