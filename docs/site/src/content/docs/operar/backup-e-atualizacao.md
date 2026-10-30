---
title: Backup e atualização
description: Como fazer backup e restaurar o Vintém, atualizar de versão e migrar do Lastro.
---

Todo o estado do Vintém está em um arquivo SQLite. Para recuperar uma instância, você
precisa de **duas coisas**:

| O quê | Onde está | Sem ela |
|---|---|---|
| O banco | `vintem.db`, no volume `dados` | Perde cobranças, contas e API keys |
| A chave mestra | `VINTEM_MASTER_KEY`, no `.env` | As credenciais dos bancos não voltam |

Guarde a chave **separada** do backup do banco, porque quem tem os dois tem as credenciais.

## Fazer backup

Com o guia de [produção](/operar/producao/), pare o Vintém, copie o volume e suba de novo.
O Caddy continua no ar, e os webhooks que chegarem nesse intervalo o banco reenvia, ou a
conciliação encontra.

```bash
docker compose stop vintem
docker run --rm -v vintem_dados:/dados:ro -v "$PWD":/backup alpine \
  tar czf /backup/vintem-$(date +%F).tar.gz -C /dados .
docker compose start vintem
```

Mande o arquivo para fora do servidor. O banco usa WAL, então não copie o `vintem.db`
com o Vintém no ar: parte dos dados pode estar no `vintem.db-wal`.

Sem Docker, faça o mesmo: pare o processo e copie `vintem.db` e, se existirem,
`vintem.db-wal` e `vintem.db-shm`.

## Restaurar

Numa instância nova, com o mesmo `.env`:

```bash
docker compose down -v
docker volume create vintem_dados
docker run --rm -v vintem_dados:/dados -v "$PWD":/backup:ro alpine \
  tar xzf /backup/vintem-2026-10-30.tar.gz -C /dados
docker compose up -d
```

:::danger
`down -v` apaga os volumes. Use só numa instância que você vai mesmo substituir.
:::

Depois de uma parada longa, rode a [conciliação](/conceitos/recebimentos-e-conciliacao/)
com uma janela maior que o tempo fora do ar:

```bash
docker compose exec -e VINTEM_CONCILIACAO_JANELA=48h vintem /vintem conciliar
```

## Atualizar de versão

1. Leia as [notas do release](https://github.com/vintemhq/vintem/releases). Antes da
   `v1.0.0`, um minor pode quebrar o contrato, e a quebra vem anotada.
2. [Faça o backup](#fazer-backup).
3. Troque `VINTEM_VERSAO` no `.env` e suba:

```bash
docker compose pull vintem
docker compose up -d vintem
curl https://vintem.suaempresa.com/health
```

As migrations rodam sozinhas quando o banco abre. Não há comando para desfazer uma
migration: para voltar de versão, restaure o backup feito antes de atualizar.

## Migrar do Lastro

O projeto se chamava Lastro até a `v0.8.0` ([ADR-009](/gerado/adr/009-nome-e-marca/)). O
banco e as credenciais continuam valendo, mas estes nomes mudaram:

| Antes | Depois | O que fazer |
|---|---|---|
| `LASTRO_*` | `VINTEM_*` | Renomeie as variáveis. A chave mestra mantém o mesmo valor |
| `lastro.db` | `vintem.db` | Renomeie o arquivo, ou defina `VINTEM_DB=lastro.db` |
| API key `lst_...` | `vtm_...` | As antigas respondem 401. Crie outras com `vintem apikey criar` |
| `ghcr.io/lastrohq/lastro` | `ghcr.io/vintemhq/vintem` | Troque a imagem |
| `@lastrohq/sdk` | `@vintemhq/sdk` | Troque o pacote. A classe `Lastro` virou `Vintem` |

:::caution
Sem o passo do `lastro.db`, o Vintém cria um `vintem.db` **vazio** e sobe sem erro. Se as
contas sumirem depois da troca, é isso: o banco antigo continua intacto ao lado.
:::

Pare o processo antes de renomear, e renomeie junto o `-wal` e o `-shm`, se existirem.

As URLs de webhook e os segredos dos destinos continuam valendo.
