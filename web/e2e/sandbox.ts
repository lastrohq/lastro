// Compila o binário com o painel embutido (rode `npm run build` antes) e sobe
// um `vintem sandbox` de verdade. O token vai para VINTEM_TOKEN.
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

export default async function () {
  const dir = mkdtempSync(join(tmpdir(), "vintem-e2e-"));
  const bin = join(dir, "vintem");
  execFileSync("go", ["build", "-o", bin, "./cmd/vintem"], { cwd: fileURLToPath(new URL("../..", import.meta.url)), stdio: "inherit" });
  const proc = spawn(bin, ["sandbox"], { cwd: dir, env: { ...process.env, VINTEM_ADDR: "127.0.0.1:18099", VINTEM_CONCILIACAO_INTERVALO: "2s" }, stdio: ["ignore", "pipe", "ignore"] });
  let banner = "";
  proc.stdout.on("data", (b) => (banner += b));
  for (let i = 0; i < 100 && !/vtm_\S+/.test(banner); i++) await new Promise((r) => setTimeout(r, 100));
  const token = banner.match(/vtm_\S+/)?.[0];
  if (!token) throw new Error("o sandbox não imprimiu a API key:\n" + banner);
  process.env.VINTEM_TOKEN = token;
  return async () => { proc.kill(); };
}
