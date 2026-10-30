---
title: Painel
description: O painel do Vintém em /painel/, embutido no binário.
---

O painel vem dentro do binário e fica em `/painel/` (no sandbox,
`http://localhost:8080/painel/`). Ele entra com uma API key e usa a mesma API
que os outros clientes, então mostra só o que os escopos da key permitem.

| Tela | O que mostra | Escopo |
|---|---|---|
| Visão geral | Recebido hoje e no mês, cobranças ativas, quanto um gateway teria cobrado | `cob:read` |
| Cobranças | Lista com filtro; detalhe com histórico e QR Code | `cob:read` |
| Webhooks | Entregas pendentes, entregues e na DLQ, com reenvio | `webhook:admin` |
| Contas | Bancos conectados e o certificado de cada um, com alerta a 30 dias do vencimento | `conta:admin` |

Para uma key só do painel, de leitura:

```bash
vintem apikey criar --nome painel --escopo cob:read --escopo webhook:admin --escopo conta:admin
```

Binários instalados com `go install` vêm sem o painel; a imagem Docker e os
binários dos releases já trazem.
