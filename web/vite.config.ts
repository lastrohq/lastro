import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// O painel é servido pelo próprio binário em /painel/ (go:embed de web/dist).
export default defineConfig({
  base: "/painel/",
  plugins: [react()],
  build: { outDir: "dist/painel", emptyOutDir: true },
  server: { proxy: { "^/(cob|cobv|pix|lastro|health)": "http://localhost:8080" } },
  test: { environment: "jsdom", include: ["test/**/*.test.{ts,tsx}"] },
});
