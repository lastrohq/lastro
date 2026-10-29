// Compila o binário com o painel embutido (rode `npm run build` antes) e sobe
// um `lastro sandbox` de verdade. O token vai para LASTRO_TOKEN.
import { execFileSync, spawn } from "node:child_process";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

export default async function () {
  const dir = mkdtempSync(join(tmpdir(), "lastro-e2e-"));
  const bin = join(dir, "lastro");
  execFileSync("go", ["build", "-o", bin, "./cmd/lastro"], { cwd: fileURLToPath(new URL("../..", import.meta.url)), stdio: "inherit" });
  const proc = spawn(bin, ["sandbox"], { cwd: dir, env: { ...process.env, LASTRO_ADDR: "127.0.0.1:18099", LASTRO_CONCILIACAO_INTERVALO: "2s" }, stdio: ["ignore", "pipe", "ignore"] });
  let banner = "";
  proc.stdout.on("data", (b) => (banner += b));
  for (let i = 0; i < 100 && !/lst_\S+/.test(banner); i++) await new Promise((r) => setTimeout(r, 100));
  const token = banner.match(/lst_\S+/)?.[0];
  if (!token) throw new Error("o sandbox não imprimiu a API key:\n" + banner);
  process.env.LASTRO_TOKEN = token;
  return async () => { proc.kill(); };
}
