import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import starlightOpenAPI, { openAPISidebarGroups } from "starlight-openapi";

// Publicado no GitHub Pages (.github/workflows/site.yml). Com domínio próprio,
// troque site/base por https://lastro.dev e "/".
export default defineConfig({
  site: "https://lastrohq.github.io",
  base: "/lastro",
  integrations: [
    starlight({
      title: "Lastro",
      description: "Gateway Pix self-hosted: sua conta no banco, sua infraestrutura, nenhuma taxa de intermediário.",
      defaultLocale: "root",
      locales: { root: { label: "Português", lang: "pt-BR" } },
      social: [{ icon: "github", label: "GitHub", href: "https://github.com/lastrohq/lastro" }],
      customCss: ["./src/styles/marca.css"],
      editLink: { baseUrl: "https://github.com/lastrohq/lastro/edit/develop/docs/" },
      plugins: [starlightOpenAPI([{ base: "api", schema: "../../api/openapi.yaml", sidebar: { label: "Referência da API" } }])],
      sidebar: [
        { label: "Começar", items: ["comecar/quickstart", "gerado/sandbox"] },
        { label: "Bancos", items: ["gerado/efi", "gerado/inter", "gerado/sicoob"] },
        { label: "Integrar", items: ["integrar/sdk", "integrar/mcp", "integrar/painel"] },
        ...openAPISidebarGroups,
        { label: "Projeto", collapsed: true, items: ["gerado/regras-de-negocio", "gerado/arquitetura", "gerado/contribuir", { label: "Decisões (ADR)", items: [{ autogenerate: { directory: "gerado/adr" } }] }] },
      ],
    }),
  ],
});
