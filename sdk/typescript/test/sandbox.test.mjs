// Contra um `lastro sandbox` de verdade: defina LASTRO_URL e LASTRO_TOKEN.
import { test } from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { Lastro, LastroError } from "../dist/index.js";

const { LASTRO_URL: url, LASTRO_TOKEN: token } = process.env;
const pular = !url || !token ? "defina LASTRO_URL e LASTRO_TOKEN (lastro sandbox)" : false;
const lastro = pular ? null : new Lastro({ url, token });

test("o exemplo do README cria uma cobrança", { skip: pular }, async () => {
  const { stdout } = await promisify(execFile)(process.execPath, [new URL("./exemplo.mjs", import.meta.url).pathname], { env: process.env });
  assert.match(stdout, /^\w{26,35} ATIVA 0002/);
});

test("cria, consulta, lista e cancela", { skip: pular }, async () => {
  const inicio = new Date(Date.now() - 60_000).toISOString();
  const cob = await lastro.cob.criar({ calendario: { expiracao: 3600 }, valor: { original: "15.00" }, chave: "sandbox@lastro.dev" }, { idempotencyKey: "sdk-" + Date.now() });
  assert.equal(cob.status, "ATIVA");
  assert.equal((await lastro.cob.consultar(cob.txid)).txid, cob.txid);
  const lista = await lastro.cob.listar({ inicio, fim: new Date(Date.now() + 60_000).toISOString() });
  assert.ok(lista.cobs.some((c) => c.txid === cob.txid));
  assert.equal((await lastro.cob.cancelar(cob.txid)).status, "REMOVIDA_PELO_USUARIO_RECEBEDOR");
});

test("a mesma chave de idempotência não cria duas cobranças", { skip: pular }, async () => {
  const corpo = { calendario: { expiracao: 3600 }, valor: { original: "16.00" }, chave: "sandbox@lastro.dev" };
  const chave = "sdk-idem-" + Date.now();
  const a = await lastro.cob.criar(corpo, { idempotencyKey: chave });
  const b = await lastro.cob.criar(corpo, { idempotencyKey: chave });
  assert.equal(a.txid, b.txid);
});

test("erro da API vira LastroError com o problema", { skip: pular }, async () => {
  await assert.rejects(lastro.cob.consultar("txidquenaoexiste000000000001"), (e) => e instanceof LastroError && e.status === 404 && typeof e.problema?.title === "string");
  const semEscopo = new Lastro({ url, token: "lst_tokeninvalido" });
  await assert.rejects(semEscopo.cob.consultar("x"), (e) => e instanceof LastroError && e.status === 401);
});
