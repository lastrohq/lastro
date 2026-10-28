# ADR-002 · SQLite por padrão, Postgres opcional

**Status:** Aceita · **Data:** 28/10/2026

## Contexto

O diferencial de adoção é "um `docker run` e funciona". Volume típico de quem
roda o Lastro (SaaS pequeno, e-commerce, ERP) é de dezenas a milhares de Pix
por dia — muito abaixo do limite de escrita do SQLite em WAL.

## Decisão

- SQLite via `modernc.org/sqlite` (Go puro, sem CGO), modo WAL, arquivo em volume.
- Acesso só pelo port `Store`; queries via `sqlc`, migrations via `goose`.
- Postgres entra na v0.3 pelo mesmo port, para quem precisa de alta disponibilidade.

## Consequências

- ➕ Binário estático, imagem distroless, zero dependência externa.
- ➕ Backup = copiar um arquivo (documentar `VACUUM INTO` / Litestream).
- ➖ Uma instância escrevendo por vez: sem réplica ativa-ativa com SQLite.
- ➖ Dois dialetos de SQL quando o Postgres chegar; a suíte do Store roda nos dois.
