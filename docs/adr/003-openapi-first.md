# ADR-003 · OpenAPI-first, compatível com a spec do BCB

**Status:** Aceita · **Data:** 28/10/2026

## Contexto

O BCB publica a API Pix como OpenAPI (`bacen/pix-api`, v2.10.0). Muita gente já
tem cliente escrito contra ela, e SDK oficial de banco segue o mesmo desenho.

## Decisão

- `api/openapi.yaml` é a fonte da verdade, derivado da spec do BCB.
- As rotas Pix (`/cob`, `/cobv`, `/pix`, `/rec`, `/cobr`...) mantêm caminho, campos e formato de erro do BCB.
- Extensões do Lastro (ex.: `expirada`, `Idempotency-Key`) são aditivas e documentadas como `x-lastro`.
- Rotas que não existem na spec (admin, contas, API keys) vivem em `/lastro/v1/...`.
- Server stubs via `oapi-codegen`; SDK TS gerado do mesmo arquivo. O CI recusa drift.

## Consequências

- ➕ Trocar o banco por Lastro é trocar a URL base e a autenticação.
- ➕ Contrato, servidor e SDK nunca divergem.
- ➖ Herdamos escolhas da spec (valores como string decimal, nomes em português).
- ➖ Atualizar para a versão nova da spec do BCB é trabalho recorrente — e conteúdo para release.
