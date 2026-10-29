// Vetor calculado à parte (hmac do Python), no formato do app.Assinar do Lastro.
import { test } from "node:test";
import assert from "node:assert/strict";
import { verificarWebhook, AssinaturaInvalida } from "../dist/index.js";

const segredo = "whsec_Y2hhdmUtZGUtdGVzdGUtZG8tbGFzdHJvLTMyYnl0ZXM=";
const corpo = '{"type":"cobranca.paga","timestamp":"2026-10-29T12:00:00Z","data":{"txid":"abc"}}';
const headers = { "webhook-id": "msg_1", "webhook-timestamp": "1793300400", "webhook-signature": "v1,uYfx++wcj86kg41EcTQl+oKOa6l+fLvHdIoZsrwhaEg=" };
const agora = new Date(1793300400 * 1000 + 30_000);

test("assinatura válida devolve o evento", async () => {
  const ev = await verificarWebhook(segredo, headers, corpo, { agora });
  assert.equal(ev.type, "cobranca.paga");
  assert.deepEqual(ev.data, { txid: "abc" });
});

test("aceita Headers e várias assinaturas, e confere a v1 certa", async () => {
  const h = new Headers({ ...headers, "webhook-signature": "v1,AAAA v1," + headers["webhook-signature"].slice(3) });
  assert.equal((await verificarWebhook(segredo, h, corpo, { agora })).type, "cobranca.paga");
});

for (const [nome, mudar] of [
  ["corpo alterado", (a) => ({ ...a, corpo: a.corpo.replace("abc", "abd") })],
  ["segredo errado", (a) => ({ ...a, segredo: "whsec_" + btoa("outra-chave") })],
  ["fora da janela de tempo", (a) => ({ ...a, agora: new Date(1793300400 * 1000 + 600_000) })],
  ["sem webhook-signature", (a) => ({ ...a, headers: { ...a.headers, "webhook-signature": undefined } })],
  ["só versão desconhecida", (a) => ({ ...a, headers: { ...a.headers, "webhook-signature": "v2," + headers["webhook-signature"].slice(3) } })],
]) {
  test(`recusa: ${nome}`, async () => {
    const a = mudar({ segredo, headers, corpo, agora });
    await assert.rejects(verificarWebhook(a.segredo, a.headers, a.corpo, { agora: a.agora }), AssinaturaInvalida);
  });
}
