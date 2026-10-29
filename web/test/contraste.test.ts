// Os tokens de texto passam WCAG AA (4,5:1) sobre --bg e --surface, nos dois temas.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(resolve(__dirname, "../src/design/tokens.css"), "utf8");
const [claro, escuro] = css.split("@media (prefers-color-scheme: dark)");
const tokens = (bloco: string) => Object.fromEntries([...bloco.matchAll(/--([a-z-]+):\s*(#[0-9a-f]{6})/g)].map((m) => [m[1], m[2]]));

function luminancia(hex: string): number {
  const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255).map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
  return 0.2126 * c[0]! + 0.7152 * c[1]! + 0.0722 * c[2]!;
}
export function contraste(a: string, b: string): number {
  const [l1, l2] = [luminancia(a), luminancia(b)].sort((x, y) => y - x);
  return (l1! + 0.05) / (l2! + 0.05);
}

const textos = ["text", "text-muted", "brand", "success", "warning", "danger", "info"];
for (const [tema, bloco] of [["claro", claro!], ["escuro", escuro!]] as const) {
  describe(`tema ${tema}`, () => {
    const t = tokens(bloco);
    for (const cor of textos) {
      for (const fundo of ["bg", "surface"]) {
        it(`--${cor} sobre --${fundo} passa AA`, () => {
          expect(contraste(t[cor]!, t[fundo]!)).toBeGreaterThanOrEqual(4.5);
        });
      }
    }
  });
}
