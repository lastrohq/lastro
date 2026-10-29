# Como contribuir com o Lastro

Obrigado por querer ajudar! O jeito de maior impacto é **adicionar ou melhorar
o suporte a um banco**: cada banco novo é um público inteiro que passa a poder
usar o Lastro.

## Antes de começar

1. Leia o [manual do projeto](./docs/EQUIPE.md) (fluxo, DoR e DoD) e as [regras de negócio](./docs/REGRAS-DE-NEGOCIO.md).
2. Procure a issue no [board](https://github.com/orgs/lastrohq/projects). Não achou? Abra uma pelo template antes de escrever código — evita PR que não entra.
3. Comente na issue que vai pegar.

## Ambiente

```bash
git clone https://github.com/lastrohq/lastro && cd lastro
git checkout develop
make test   # go test -race ./...
make lint   # golangci-lint run
make run    # sobe em :8080
```

Requisitos: Go 1.27+, [golangci-lint](https://golangci-lint.run/) v2, Docker (para a imagem).

## Fluxo

- Branch a partir de `develop`: `feature/descricao-curta` ou `fix/descricao-curta`.
- Commits em [Conventional Commits](https://www.conventionalcommits.org/), em português: `feat: adiciona adapter do Sicoob`.
- `Closes #N` na mensagem do commit.
- PR para `develop`. CI verde é obrigatório.

## Escrevendo um adapter de banco

1. Abra (ou pegue) a issue pelo template **🏦 Novo adapter de banco**.
2. Crie `internal/adapters/psp/<banco>/` implementando o port `PSP` (`internal/ports`).
3. Declare em `Capabilities()` só o que o banco realmente suporta.
4. Trate **dentro do adapter** todo desvio do banco em relação à spec do BCB, com um comentário dizendo qual desvio e a fonte (link da doc do banco).
   - **Token OAuth2:** não peça token a cada chamada. Escreva só a função que busca o token no banco (o formato muda de banco para banco; `oauth.LerResposta` lê a resposta padrão) e use o `oauth.Transport` de `internal/platform/oauth` no `http.Client`, com o transporte mTLS da conta como `Base`. Ele guarda o token por conta, faz uma só busca com chamadas concorrentes, renova antes de expirar e tenta de novo uma vez com token novo se o banco responder 401.
5. Grave fixtures da homologação do banco em `test/fixtures/<banco>/` — **sem segredo e sem dado real** (o gravador mascara headers de autenticação; confira mesmo assim).
6. Rode a suíte de conformidade: `go test ./test/conformance -run <Banco>`.
7. Atualize a tabela de bancos suportados no README.

Adapter só entra se passar na suíte de conformidade.

## O que não entra

Para manter o projeto sustentável, o escopo é fechado: cobrança, devolução, Pix
Automático, webhooks, conciliação, painel e DX. Boleto, cartão, split,
antecipação e emissão de nota ficam de fora — abra uma Discussion se quiser
argumentar.

## Segurança

Nunca abra issue pública para vulnerabilidade. Veja o [SECURITY.md](./SECURITY.md).
