# ADR-001 · Arquitetura hexagonal com adapter por PSP

**Status:** Aceita · **Data:** 28/10/2026

## Contexto

Todo banco implementa a API Pix do BCB com desvios próprios (autenticação,
formato de erro, campos extras, webhooks com ou sem mTLS). O valor do Lastro é
justamente esconder esses desvios. E o crescimento do projeto depende de
pessoas de fora conseguirem adicionar bancos sem entender o núcleo.

## Decisão

- Domínio e casos de uso não conhecem banco nenhum.
- Cada banco é um pacote em `internal/adapters/psp/<banco>` que implementa o port `PSP` e declara `Capabilities`.
- Adapters se registram num registry por nome; a conta diz qual usar.
- Todo adapter passa pela mesma suíte de conformidade (`test/conformance`).

## Consequências

- ➕ Adapter novo é uma PR isolada, testável contra fixtures, ideal para *good first issue*.
- ➕ Desvio de banco fica confinado — o domínio nunca ganha `if banco == ...`.
- ➖ Mais interfaces e mapeamento do que um cliente HTTP direto.
- ➖ O port `PSP` precisa ser estável cedo; mudar depois custa em todos os adapters.
