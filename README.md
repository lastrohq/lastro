<div align="center">

# Lastro

### Pix without a gateway. 0% fees, your business bank account, your infrastructure.

**Self-hosted, multi-bank Pix gateway with Pix Automático (recurring Pix) from day one.**
One Docker image that speaks the Central Bank's Pix API, ships an adapter per bank,
handles the mTLS certificates and normalizes webhooks.

[Português](./README.pt-BR.md) · [Docs](https://lastro.dev/docs) · [Roadmap](https://github.com/orgs/lastrohq/projects)

</div>

> 🚧 **Pre-alpha.** Nothing here works yet. Watch the repo or follow the board.

## Why

Payment gateways charge ~1% or more per Pix — on a payment rail that costs the
merchant almost nothing at the bank. Every bank already exposes the official
[BCB Pix API](https://github.com/bacen/pix-api), but each one deviates from it,
requires its own mTLS certificate dance, and sends webhooks in its own shape.

Lastro is the missing layer: you run it, you plug in your own bank credentials,
and your app talks to **one** API no matter which bank receives the money.

## What it will do (MVP)

- **One API, many banks** — Efí, Inter, Sicoob and Banco do Brasil first; adapters are community-friendly.
- **Charges** — immediate (`cob`) and with due date (`cobv`), BR Code / QR generation built in.
- **Pix Automático** — `rec`, `solicrec`, `cobr` and recurring webhooks.
- **Webhooks that don't get lost** — normalized, signed ([Standard Webhooks](https://www.standardwebhooks.com/)), retried, plus a reconciliation job that recovers missed payments.
- **Stateful sandbox** — test the full flow without a business bank account.
- **Single binary** — `docker run` and go; SQLite by default.
- **Use it from your agent** — built-in MCP server.

## Banks

| Bank | `--psp` | cob | cobv | Refund | Webhook | Status |
|---|---|---|---|---|---|---|
| Lastro sandbox | `sandbox` | ✅ | ✅ | ✅ | ✅ | For testing without a business account ([guide, pt-BR](./docs/SANDBOX.md)) |
| Efí | `efi` | ✅ | ✅ | ✅ | ✅ | **Not yet tested against Efí's homologation environment** ([guide, pt-BR](./docs/EFI.md)) |
| Inter, Sicoob, Banco do Brasil | — | | | | | Planned |

## Non-custodial

Lastro never holds or moves money. It runs with **your** credentials against
**your** bank account. This is not legal or regulatory advice.

## License

[Apache-2.0](./LICENSE). "Pix" is a trademark of Banco Central do Brasil;
Lastro is an independent project, not affiliated with or endorsed by the BCB.
