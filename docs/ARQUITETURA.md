# 🏗️ Lastro — Arquitetura

## Em uma tela

Binário Go único, **hexagonal** (ports & adapters). O domínio não conhece banco,
HTTP nem storage. Cada PSP é um adapter que implementa o mesmo port e passa pela
mesma suíte de conformidade — é isso que permite adapter escrito pela
comunidade sem medo.

```
 App do cliente ──REST (BCB-compatível + Idempotency-Key)──▶ ┌─────────────── lastro ───────────────┐
 Agente de IA  ──MCP──────────────────────────────────────▶ │ httpapi · admin · mcp · painel(embed)│
                                                             │          ▼ app (casos de uso)        │
 App do cliente ◀── webhook normalizado e assinado ──────────│ domain: cobrança, devolução, rec...  │
                                                             │ ports: PSP · Store · Notifier · Vault│
                                                             │ psp: efi│inter│sicoob│bb│sandbox     │
                                                             └──── mTLS + OAuth2 ──▶ API Pix do banco
                                                                    ◀── webhook do banco
```

## Pastas

| Caminho | O que vive ali | Pode importar |
|---|---|---|
| `cmd/lastro` | `main`: monta o registry de PSPs e chama a CLI | tudo |
| `internal/cli` | Comandos (`serve`, `conta`, `chave`, `version`...), testáveis sem processo; monta as peças | tudo, menos `cmd` |
| `internal/domain` | Entidades, máquinas de estado, `Money`, `Txid` | só stdlib |
| `internal/app` | Casos de uso (`CriarCobranca`, `Devolver`, `CriarRecorrencia`...) | `domain`, `ports` |
| `internal/ports` | Interfaces `PSP`, `Store`, `Notifier`, `Vault`, `Certificados`, `Credenciais`, `Clock` | `domain` |
| `internal/adapters/httpapi` | API BCB-compatível, admin, webhooks de entrada | `app`, `ports`, `domain` |
| `internal/adapters/psp/*` | Um pacote por banco + `sandbox` | `ports`, `domain`, `platform`, o registry `adapters/psp`, `pkg` |
| `internal/adapters/store/*` | `sqlite` (padrão), `postgres` (v0.3) | `ports`, `domain` |
| `internal/adapters/notifier` | Outbox, entrega, retry, DLQ, assinatura | `ports`, `domain` |
| `internal/adapters/mcp` | Ferramentas MCP sobre a API HTTP, com a API key do agente ([ADR-008](adr/008-mcp-como-cliente-da-api.md)) | SDK MCP, `net/http` |
| `internal/adapters/jobs` | Laço periódico (conciliação, entrega, limpezas) | só stdlib/libs |
| `internal/platform` | `vault` (AES-256-GCM), `oauth` (cache de token OAuth2 por conta), `certificado` (leitura de .p12/PEM), e depois mTLS | só stdlib/libs — a checagem `var _ ports.Vault` fica no teste |
| `internal/adapters/psp` | `Registry` + um subpacote por banco | `ports`, `domain` |
| `pkg/brcode` | Lib pública de BR Code | só stdlib |
| `api/openapi.yaml` | Contrato da API (fonte da verdade) | — |
| `web/` | Painel React + design system | — |
| `test/conformance` | Suíte que todo adapter de PSP precisa passar | — |
| `test/e2e` | Binário em modo sandbox, fluxo de ponta a ponta | — |

**A regra de dependência é verificada no CI** pelo `depguard`, configurado no
[`.golangci.yml`](../.golangci.yml) a partir desta tabela:

- `domain` e `pkg` só importam a stdlib, **inclusive nos testes**.
- Nas outras camadas, a regra vale para o **código de produção**. Teste pode
  montar integração real: os testes de `app` e `httpapi` usam o sandbox, o
  notifier e o vault de verdade.
- Um adapter não importa outro adapter. As exceções são o registry
  `adapters/psp` para os bancos e os subpacotes do próprio adapter, como
  `httpapi/gen` e `store/sqlite/db`.

Mudou a tabela? Mude o `.golangci.yml` no mesmo PR.

## Stack

| Peça | Escolha | Por quê |
|---|---|---|
| Linguagem | Go 1.27+ | Binário único, imagem pequena, mTLS de primeira na stdlib |
| HTTP | `net/http` com roteamento nativo | Sem framework para manter |
| Contrato | OpenAPI-first + `oapi-codegen` | A spec do BCB já é OpenAPI — ver [ADR-003](./adr/003-openapi-first.md) |
| Storage | SQLite (`modernc.org/sqlite`, sem CGO, WAL) | `docker run` sem dependência — ver [ADR-002](./adr/002-sqlite-por-padrao.md) |
| SQL | `sqlc` (via `go tool`) + `goose` com migrations embutidas | Query tipada, migration versionada. `make generate` regera; o CI roda `sqlc diff` |
| Fila | Tabela outbox + workers | Sem Redis; transação única com a escrita |
| Log | `slog` (JSON) | stdlib |
| Painel | React + Vite + TS, via `go:embed` | Continua binário único |
| Release | goreleaser → binários + `ghcr.io/lastrohq/lastro` multi-arch | |
| SDK | `@lastro/sdk` gerado do OpenAPI | |
| MCP | `modelcontextprotocol/go-sdk` | |

## Padrões

- **Adapter / Strategy** — um `PSP` por banco, escolhido pela conta.
- **Registry** — adapters se registram por nome (`psp.Register("efi", ...)`); adicionar banco não mexe no núcleo.
- **Capabilities** — cada adapter declara o que suporta ([RN-50](./REGRAS-DE-NEGOCIO.md#rn-50--matriz-de-capacidades)).
- **Máquina de estados explícita** — transições como tabela no domínio; teste table-driven cobre todas as transições, válidas e inválidas.
- **Repository** — `Store` por agregado, transação via `Store.WithTx`.
- **Outbox** — evento gravado na mesma transação da mudança de estado; worker entrega.
- **Idempotency Key** — middleware no `httpapi` + tabela própria.
- **Clock e IDs injetáveis** — testes determinísticos, sem `time.Now()` no domínio.
- **Functional options** — na configuração de adapters e do servidor.
- **Erros de domínio tipados** — `domain.ErrTransicaoInvalida` etc., mapeados para o formato de erro da spec do BCB (`type`, `title`, `status`, `detail`, `violacoes`) num lugar só.

## Ports

Definidos em `internal/ports` (#11):

| Port | O que faz |
|---|---|
| `PSP` | Adapter do banco: `CriarCob`, `ConsultarCob`, `ListarPix`, `Devolver`, `ConfigurarWebhook`, mais `Capabilities()` (RN-50) |
| `PSPRecorrencia` | Parte **opcional** do PSP para Pix Automático: `CriarRecorrencia`, `ConsultarRecorrencia`, `SolicitarConfirmacao`. Só o adapter que declara `CapRec` a implementa; o caso de uso confere as duas coisas e responde `ErrNaoSuportado` se faltar uma |
| `PSPCobrancaRecorrente` | Parte **opcional** do PSP para a cobrança recorrente: `CriarCobr`, `ConsultarCobr`, `CancelarCobr`, `SolicitarRetentativa`. Mesma regra, com `CapCobr` |
| `Store` | `Contas()`, `Cobrancas()`, `Outbox()` e `WithTx`. Atualização com **revisão otimista**: escrita sobre revisão velha falha com `ErrConflito` |
| `OutboxRepo` | É o lado "publicar" do Notifier: o evento entra na mesma transação da mudança de estado (ADR-005). A entrega fica no adapter `notifier` |
| `Clock` | Fonte de tempo injetável |
| `Vault` | Cifrar/decifrar segredos em repouso (RN-61) |
| `Certificados` | Ler o certificado do mTLS (.p12/PEM) e devolver os dados públicos dele (RN-62). Implementado sobre `platform/certificado`, montado na CLI |
| `Credenciais` | Entregar ao adapter de PSP as credenciais decifradas de uma conta. Implementado por `app.Contas`; o registry é montado depois do vault ([ADR-007](./adr/007-credenciais-chegam-ao-adapter-por-port.md)) |

Fakes em memória ficam em `internal/ports/fake`. **`internal/ports/storetest` é a suíte de
contrato do Store**: o fake passa nela hoje, e o SQLite (#12) e o Postgres precisam passar nos
mesmos testes.

## Testes

| Camada | Como | Onde |
|---|---|---|
| Domínio | Table-driven, 100% das transições de estado | `internal/domain` |
| BR Code | Vetores *golden* de bancos reais, `go test -fuzz` no parser, round-trip gerar→ler | `pkg/brcode` |
| Casos de uso | Fakes dos ports + clock fixo | `internal/app` |
| PSP | **Suíte de conformidade**: mesmos cenários para todo adapter — contra o sandbox sempre, contra fitas gravadas da homologação (`test/conformance/fita`) no CI, contra a homologação do banco manualmente | `test/conformance` |
| Store | Integração em SQLite real (arquivo temporário); Postgres via testcontainers quando entrar | `internal/adapters/store` |
| HTTP | `httptest` contra o roteador real, com **toda resposta validada contra o `openapi.yaml`** (kin-openapi). Novo handler ganha teste via `conferirResposta` | `internal/adapters/httpapi` |
| E2E | Binário em modo sandbox: cria cobrança → paga → webhook chega num servidor de teste com assinatura válida | `test/e2e` |
| Painel | Vitest (unidade) + Playwright (fluxos críticos) | `web/` |

**CI — check `Lint & Test`** (obrigatório em `develop` e `main`): `golangci-lint`,
`go test -race`, `govulncheck`, build da imagem. Entram conforme as fases:
cobertura mínima de 80% em `domain`, `app` e `pkg/brcode`; drift de `sqlc` e
`oapi-codegen`; lint/test/build do painel.

**Fixtures nunca têm dado real.** CPF/CNPJ de fixture são gerados válidos e
fictícios; certificado de teste é autoassinado e gerado no próprio teste.
