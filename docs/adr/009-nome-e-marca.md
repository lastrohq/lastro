# ADR-009 · Nome do projeto e uso da marca Pix

**Status:** Proposta (a decisão do nome é do mantenedor) · **Data:** 30/10/2026

## Contexto

A #2 pediu, antes do lançamento, duas respostas: se o nome **Lastro** está livre
no INPI e como usar "Pix" sem ferir a marca do BCB. Descobrir conflito de marca
depois do lançamento custa o nome inteiro, com estrelas, links e pacotes juntos.

### O que o INPI mostra para "Lastro"

Busca exata no [pePI](https://busca.inpi.gov.br/pePI/) em 30/10/2026: 49
processos. Os que importam, por estarem em vigor ou em exame nas classes de
software e de serviços financeiros:

| Processo | Classe | Situação | Titular | O que cobre |
|---|---|---|---|---|
| 927456176 | 42 | Em vigor até 22/07/2035 | LASTRO TECNOLOGIA FINANCEIRA E IMOBILIÁRIA LTDA | Elaboração de software, programação de computador, software como serviço (SaaS) |
| 927520052 | 42 | Em vigor até 11/11/2035 (mista) | LASTRO TECNOLOGIA FINANCEIRA E IMOBILIÁRIA LTDA | Mesma classe |
| 943868149, 943868270 | 09 | Aguardando exame (depósito 26/05/2026) | LASTRO TECNOLOGIA FINANCEIRA E IMOBILIÁRIA LTDA | Programas de computador, plataforma de IA |
| 943846498, 943846668 | 35 | Aguardando exame (depósito 22/05/2026) | LASTRO TECNOLOGIA FINANCEIRA E IMOBILIÁRIA LTDA | — |
| 817627006 | 36 | Em vigor até 27/03/2031 (mista) | LASTRO RDV DISTRIB DE TÍTULOS E VALORES MOBS LTDA | Corretagem de seguros, **cobrança** e avaliações financeiras |

A LASTRO TECNOLOGIA FINANCEIRA E IMOBILIÁRIA é a proptech Lastro, que já fez
gestão de aluguéis e cobranças e hoje vende IA para imobiliárias, com Série A de
R$ 85 milhões liderada pela Prosus ([Startups](https://startups.com.br/negocios/proptechs/proptech-lastro-levanta-serie-a-de-r-85m-liderada-pela-prosus/)).
O domínio `lastro.dev` também tem dono (registrado em 29/07/2026), e
`lastro.com.br` é de outra empresa desde 1996.

Um software de cobrança por Pix chamado Lastro cai nas classes 42 e 09 de uma
empresa com dinheiro para defender a marca, e na 36 de outra que cobre
"cobrança". Isto não é parecer jurídico, mas o risco não é pequeno.

### Nomes alternativos, numa primeira triagem

Busca exata no INPI (marcas não extintas nas classes 09, 35, 36, 38 e 42),
domínio `.dev` (RDAP) e GitHub, em 30/10/2026:

| Nome | INPI nas classes de software e finanças | `.dev` | Org no GitHub |
|---|---|---|---|
| Vintém | nenhuma | livre | `vintem` ocupado; `vintemhq` a conferir |
| Tostão | nenhuma | livre | `tostao` ocupado; `tostaohq` a conferir |
| Pataca | nenhuma | livre | `pataca` ocupado |
| Mirréis | nenhuma | livre | `mirreis` livre |
| Quita | Cappta S.A. (36 e 42) | ocupado | ocupado |
| Cobre | classe 35 | ocupado | ocupado |
| Liquida | classe 35 | livre | ocupado |
| Repasse | classe 35 | livre | ocupado |

A triagem é por nome exato. Antes de escolher, vale buscar também por radical
e por semelhança fonética no pePI, e, se o projeto for virar empresa, depositar
o pedido de marca.

### O que o Manual de Uso da Marca Pix permite

[Manual de Uso da Marca Pix](https://www.bcb.gov.br/content/estabilidadefinanceira/pix/Regulamento_Pix/I_manual_uso_marca_pix.pdf),
versão 1.6 (14/05/2025):

- A marca é de titularidade exclusiva do BCB, que licencia os participantes do
  Pix. O Lastro não é participante.
- "Pix" **não entra em nome composto** nem como prefixo ou sufixo de marca nova
  (p. 29, p. 34 itens 11 e 12). O nome do projeto não pode ter "Pix".
- Usar "Pix" em texto para informar sobre o meio de pagamento é admitido.
- Em texto: **Pix**, com a primeira maiúscula; todo em maiúsculas só se o texto
  inteiro estiver assim; sem aspas, itálico ou sublinhado (p. 30).
- O símbolo e o logotipo não podem ser redesenhados nem usados para criar marca
  de terceiro (p. 33 e 34).

## Decisão

Regra para o README, as docs, o painel e o site, valendo já:

1. O nome do projeto não tem "Pix", e nenhum produto do projeto também.
2. "Pix" aparece só para dizer o que o software faz, grafado **Pix**, sem
   aspas ou itálico, nunca "PIX" em texto misto nem "pix" em prosa.
3. Nenhum símbolo ou logotipo do Pix no projeto, nem redesenhado.
4. O aviso de marca fica no fim dos READMEs: Pix é marca do BCB e o Lastro é
   independente, sem vínculo nem endosso do BCB.

**O nome fica para o mantenedor decidir antes do lançamento (#42).** A
recomendação é trocar antes de o projeto ganhar estrelas e links. Hoje a troca
é barata: org e repositório no GitHub (o GitHub redireciona o antigo), módulo
Go, nome do binário, imagem no ghcr.io e site. O SDK ainda não saiu no npm.

## Consequências

- **Melhor:** o lançamento não carrega um conflito de marca conhecido, e as
  docs seguem o manual do BCB.
- **Pior:** trocar o nome mexe em tudo que tem "lastro": módulo Go, imports,
  binário, org, imagem, site, README, board e textos do lançamento.
- O `lastro.dev` sai de qualquer plano; o domínio acompanha o nome escolhido.
