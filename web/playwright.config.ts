import { defineConfig } from "@playwright/test";

// Usa o Google Chrome instalado (channel "chrome"): nada de baixar navegador.
export default defineConfig({
  testDir: "e2e",
  timeout: 90_000,
  globalSetup: "./e2e/sandbox.ts",
  use: { baseURL: "http://127.0.0.1:18099", channel: "chrome", headless: true },
  reporter: [["list"]],
});
