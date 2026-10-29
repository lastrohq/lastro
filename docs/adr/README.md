# ADRs — decisões de arquitetura

Uma decisão por arquivo, numerada e nunca reescrita. Mudou de ideia? ADR nova
com `Status: substitui ADR-00X`, e a antiga ganha `Status: substituída por ADR-00Y`.

| # | Decisão | Status |
|---|---|---|
| [001](./001-arquitetura-hexagonal.md) | Arquitetura hexagonal com adapter por PSP | Aceita |
| [002](./002-sqlite-por-padrao.md) | SQLite por padrão, Postgres opcional | Aceita |
| [003](./003-openapi-first.md) | OpenAPI-first, compatível com a spec do BCB | Aceita |
| [004](./004-nao-custodial.md) | O Lastro é não custodial | Aceita |
| [005](./005-standard-webhooks.md) | Webhooks de saída no padrão Standard Webhooks, com outbox | Aceita |
| [006](./006-certificado-conferido-no-cadastro.md) | Certificado do mTLS conferido no cadastro, com go-pkcs12 | Aceita |
| [007](./007-credenciais-chegam-ao-adapter-por-port.md) | Credenciais chegam ao adapter de PSP por um port, montado depois do vault | Aceita |
| [008](./008-mcp-como-cliente-da-api.md) | O servidor MCP é um cliente da API HTTP, não um atalho para o banco | Aceita |

Modelo: **Contexto** (o que força a decisão) · **Decisão** · **Consequências** (o que fica melhor, o que fica pior).
