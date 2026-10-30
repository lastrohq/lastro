import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import starlightOpenAPI, { openAPISidebarGroups } from "starlight-openapi";

// Publicado no Cloudflare Workers pelo Workers Builds (wrangler.jsonc), na
// raiz de https://vintem.dev.
export default defineConfig({
  site: "https://vintem.dev",
  base: "/",
  integrations: [
    starlight({
      title: "Vintém",
      description: "Gateway Pix self-hosted: sua conta no banco, sua infraestrutura, nenhuma taxa de intermediário.",
      defaultLocale: "root",
      locales: { root: { label: "Português", lang: "pt-BR" } },
      social: [{ icon: "github", label: "GitHub", href: "https://github.com/vintemhq/vintem" }],
      customCss: ["./src/styles/marca.css"],
      editLink: { baseUrl: "https://github.com/vintemhq/vintem/edit/develop/docs/" },
      plugins: [starlightOpenAPI([{ base: "api", schema: "../../api/openapi.yaml", sidebar: { label: "Referência da API" } }])],
      sidebar: [
        { label: "Começar", items: ["comecar/quickstart", "gerado/sandbox"] },
        { label: "Bancos", items: ["gerado/efi", "gerado/inter", "gerado/sicoob", "gerado/bb"] },
        { label: "Integrar", items: ["integrar/sdk", "integrar/mcp", "integrar/painel"] },
        ...openAPISidebarGroups,
        { label: "Projeto", collapsed: true, items: ["gerado/regras-de-negocio", "gerado/arquitetura", "gerado/contribuir", { label: "Decisões (ADR)", items: [{ autogenerate: { directory: "gerado/adr" } }] }] },
      ],
    }),
  ],
});
