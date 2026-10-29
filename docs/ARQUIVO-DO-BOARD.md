<div align="center">

# 🗄️ Lastro — Arquivo do board

### O que já foi entregue, com o caminho de volta ao código.

<sub>📋 board no <a href="https://github.com/orgs/lastrohq/projects">GitHub Projects</a> &nbsp;•&nbsp; 🧭 processo no <a href="./EQUIPE.md">manual do projeto</a></sub>

</div>

---

## Por que este arquivo existe

Card arquivado some do board. A issue continua fechada e o commit continua no
histórico, mas a linha que liga *"o que foi combinado"* a *"o commit que
entregou"* só existe enquanto alguém lembra. Este arquivo guarda essa linha.

O board é a fonte da verdade do **agora**; este documento é a do **já foi**.

### Como arquivar

Ao fim da sprint, para cada card em `✅ Concluído`:

1. Confirme que a issue foi fechada pelo `Closes` no merge;
2. Ache o **PR que fechou** e o **release** que levou aquilo para a `main`;
3. Registre a linha na tabela da leva;
4. **Só então** arquive o card (`gh project item-archive <n> --owner lastrohq --id <id>`).

### Épico que fecha ganha seção própria

Épico não tem PR nem release. O que vale registrar dele é a leitura de
conjunto: o que era, o que virou, as decisões que deixou e o que ensinou.

---

## Leva de 28/10/2026 — v0.1.0: do repositório vazio à primeira cobrança paga no sandbox

O primeiro release, e o dia inteiro dele: **F0, F1 e F2 fecharam juntas**. O
Lastro saiu de um PDF de ideias para um binário que cria cob e cobv pela API
compatível com o BCB, autentica por API key, deduplica por Idempotency-Key e
publica imagem multi-arch no `ghcr.io`.

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|
| [#3](https://github.com/lastrohq/lastro/issues/3) | main e develop protegidas com o check Lint & Test | Infra/CI | P | Alta | — (via API, comentado na issue) | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 18:24 |
| [#5](https://github.com/lastrohq/lastro/issues/5) | Release com tag publica binários e imagem multi-arch no ghcr.io | Infra/CI | M | Média | [#56](https://github.com/lastrohq/lastro/pull/56) `8695c28` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 21:49 |
| [#6](https://github.com/lastrohq/lastro/issues/6) | Valores em dinheiro nunca perdem centavo (Money em int64) | Core | P | Alta | [#44](https://github.com/lastrohq/lastro/pull/44) `9ea705b` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 18:54 |
| [#7](https://github.com/lastrohq/lastro/issues/7) | txid gerado e validado conforme a spec (26–35 alfanuméricos) | Core | P | Alta | [#44](https://github.com/lastrohq/lastro/pull/44) `9ea705b` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 18:54 |
| [#8](https://github.com/lastrohq/lastro/issues/8) | Cobrança só muda de estado por transição permitida | Core | M | Alta | [#45](https://github.com/lastrohq/lastro/pull/45) `2b4c046` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 18:57 |
| [#10](https://github.com/lastrohq/lastro/issues/10) | Ler um BR Code "Pix Copia e Cola" e saber o que ele cobra | Core | M | Média | [#46](https://github.com/lastrohq/lastro/pull/46) `0b30133` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 19:00 |
| [#11](https://github.com/lastrohq/lastro/issues/11) | Ports PSP, Store, Notifier, Vault e Clock definidos | Core | M | Alta | [#47](https://github.com/lastrohq/lastro/pull/47) `bffc2aa` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 19:04 |
| [#12](https://github.com/lastrohq/lastro/issues/12) | Dados do Lastro persistem em SQLite com migrations versionadas | Core | M | Alta | [#48](https://github.com/lastrohq/lastro/pull/48) `717b6e5` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 20:04 |
| [#13](https://github.com/lastrohq/lastro/issues/13) | Contrato da API em api/openapi.yaml derivado da spec do BCB | API | M | Alta | [#49](https://github.com/lastrohq/lastro/pull/49) `f472593` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 20:23 |
| [#18](https://github.com/lastrohq/lastro/issues/18) | Uma instância atende várias contas recebedoras | Core | M | Alta | [#50](https://github.com/lastrohq/lastro/pull/50) `188ca42` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 20:34 |
| [#14](https://github.com/lastrohq/lastro/issues/14) | Criar e consultar cobrança imediata (cob) pela API | API | G | Alta | [#52](https://github.com/lastrohq/lastro/pull/52) `656ad18` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 20:49 |
| [#16](https://github.com/lastrohq/lastro/issues/16) | Acesso à API só com API key de escopo limitado | Segurança | M | Alta | [#53](https://github.com/lastrohq/lastro/pull/53) `7ac80e2` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 20:58 |
| [#17](https://github.com/lastrohq/lastro/issues/17) | Reenviar a mesma criação não gera cobrança duplicada (Idempotency-Key) | API | M | Alta | [#54](https://github.com/lastrohq/lastro/pull/54) `2582256` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 21:05 |
| [#15](https://github.com/lastrohq/lastro/issues/15) | Criar e consultar cobrança com vencimento (cobv) pela API | API | G | Média | [#55](https://github.com/lastrohq/lastro/pull/55) `ed0eec2` | [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) · [#57](https://github.com/lastrohq/lastro/pull/57) `7a0a897` | 28/10 21:23 |

> `develop` e `main` com o mesmo conteúdo depois do [#57](https://github.com/lastrohq/lastro/pull/57);
> release [v0.1.0](https://github.com/lastrohq/lastro/releases/tag/v0.1.0) com 6 binários, checksums e imagem
> `ghcr.io/lastrohq/lastro:0.1.0` (amd64 e arm64).
>
> **Ainda não ⚠️ — imagem privada.** Pacote novo no `ghcr.io` nasce privado, e a API
> do GitHub não muda visibilidade de pacote: é na interface da org. Até lá,
> `docker pull` anônimo responde `unauthorized`. Atualizar esta linha quando abrir.

### O contrato pegou o que o teste de unidade não pegava

A decisão de **validar toda resposta HTTP contra o `openapi.yaml`** (kin-openapi)
pagou duas vezes no mesmo dia: header obrigatório ausente respondia `text/plain`
em vez de `Problema` (#14), e a ausência de uma resposta `default` teria escondido
os 502 de PSP do contrato. Nenhum teste de unidade olhava para isso.

### Três desvios da spec, corrigidos antes de sair

Conferir a spec do BCB, e não a memória, mudou três regras já escritas:
`revisao` começa em **0**, a expiração padrão da cob é **86400 s** (não 3600) e o
**CNPJ é alfanumérico**. Viraram um commit `fix:` separado e notas na RN-13/RN-14.

### O CI que nunca rodou

O primeiro PR ficou sem check: o workflow só existia nas branches de feature, e a
`develop` protegida exigia um check que nunca aparecia. Saiu com **um desvio único
autorizado** — proteção desligada, merge do #43, proteção religada no mesmo
comando. Fica registrado para não repetir: **workflow de CI entra na branch padrão
antes de ela ser protegida.**

### Go 1.26.0 tinha 23 vulnerabilidades

`go 1.26.0` no `go.mod` fazia o `setup-go` instalar exatamente o `.0`. O
`govulncheck` reprovou o #48, e o CI passou a usar o último patch (`1.26.x`), com
`toolchain go1.26.10` no `go.mod` para quem roda local.

### Ficou aberto

- [#19](https://github.com/lastrohq/lastro/issues/19) — sandbox ainda não dispara webhook nem persiste estado (F4).
- [#21](https://github.com/lastrohq/lastro/issues/21) — suíte de conformidade espera as fixtures do primeiro banco real (F5).
- [#9](https://github.com/lastrohq/lastro/issues/9) — BR Code ainda sem teste manual em apps de banco.

---

<!-- Modelo de leva:

## Leva de DD/MM/AAAA — título que diz o que mudou

| Issue | Título | Área | Est. | Prior. | PR → `develop` | Release → `main` | Issue fechada |
|---|---|---|---|---|---|---|---|

-->

