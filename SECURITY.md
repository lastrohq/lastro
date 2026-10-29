# Política de segurança

O Lastro lida com credenciais bancárias e certificados de quem o roda. Levamos
isso a sério.

## Como reportar

**Não abra issue pública.** Use o [reporte privado de vulnerabilidade do GitHub](https://github.com/lastrohq/lastro/security/advisories/new).

Inclua: versão (`lastro version`), passos para reproduzir e impacto esperado.
**Nunca envie certificado, client secret ou dado real de pagador**, nem no reporte privado.

## O que esperar

- Confirmação de recebimento em até **3 dias úteis**.
- Avaliação inicial e plano em até **10 dias úteis**.
- Correção publicada como release de segurança, com crédito a quem reportou (se quiser).

## Versões suportadas

Antes da `v1.0.0`, só a versão mais recente recebe correções de segurança.

## Escopo

Em escopo: o código deste repositório, a imagem `ghcr.io/lastrohq/lastro` e o SDK `@lastro/sdk`.
Fora de escopo: a API dos bancos, a infraestrutura de quem roda o Lastro e o sandbox público de demonstração.
