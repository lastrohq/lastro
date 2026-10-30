// Telas de Webhooks e Contas (#36). A API dessas rotas é coberta pelos testes
// em Go (entregas_test.go); aqui as respostas são simuladas para ter DLQ e
// certificado vencendo, o que um sandbox novo não tem.
import { expect, test, type Page } from "@playwright/test";

const agora = new Date();
const em = (dias: number) => new Date(agora.getTime() + dias * 86_400_000).toISOString();
const entrega = (status: string, tentativas: number, extra = {}) => ({
  eventoId: "evt_123", destinoId: "dst_1", contaId: "loja", tipo: "cobranca.concluida", status, tentativas,
  criadaEm: em(-1), proximaEm: em(0), atualizadaEm: em(0), ...extra,
});

async function entrar(page: Page) {
  await page.goto("/painel/");
  await page.getByLabel("API key").fill(process.env.VINTEM_TOKEN!);
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { name: "Visão geral" })).toBeVisible();
}

test("reenviar da DLQ: a entrega sai da DLQ e a nova tentativa aparece", async ({ page }) => {
  let reenviada = false;
  await page.route("**/vintem/v1/webhooks/entregas?*", (r) => {
    const status = new URL(r.request().url()).searchParams.get("status");
    const lista = status === "dlq" && !reenviada ? [entrega("dlq", 9, { ultimoHttp: 500, ultimoErro: "status 500" })]
      : status === "entregue" && reenviada ? [entrega("entregue", 1, { ultimoHttp: 204 })] : [];
    return r.fulfill({ json: { entregas: lista } });
  });
  await page.route("**/vintem/v1/webhooks/entregas/evt_123/reenviar", (r) => {
    expect(r.request().method()).toBe("POST");
    reenviada = true;
    return r.fulfill({ json: { reenviadas: 1 } });
  });
  await page.route("**/vintem/v1/contas", (r) => r.fulfill({ json: { contas: [] } }));

  await entrar(page);
  await expect(page.getByText("Webhooks na DLQ").locator("..")).toContainText("1");
  await page.getByRole("link", { name: "Webhooks", exact: true }).focus();
  await page.keyboard.press("Enter");
  const linha = page.getByRole("row").filter({ hasText: "evt_123" });
  await expect(linha).toContainText("HTTP 500");
  await expect(linha).toContainText("Na DLQ");
  await page.getByRole("button", { name: "Reenviar cobranca.concluida evt_123" }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("status")).toContainText("1 entrega voltou para a fila");
  await expect(page.getByText("Nenhuma entrega na DLQ")).toBeVisible();
  await page.getByLabel("Entregues").focus();
  await page.keyboard.press("Space");
  await expect(page.getByRole("row").filter({ hasText: "evt_123" })).toContainText("HTTP 204");
});

test("certificado vencendo aparece em destaque", async ({ page }) => {
  await page.route("**/vintem/v1/webhooks/entregas?*", (r) => r.fulfill({ json: { entregas: [] } }));
  await page.route("**/vintem/v1/contas", (r) => r.fulfill({ json: { contas: [
    { id: "loja-efi", nome: "Loja Efí", psp: "efi", chave: "a1b2", ambiente: "producao", ativa: true, certificado: { sujeito: "CN=loja", impressaoDigital: "ab".repeat(32), validade: em(12) } },
    { id: "loja-bb", nome: "Loja BB", psp: "bb", chave: "c3d4", ambiente: "producao", ativa: true, certificado: { sujeito: "CN=bb", impressaoDigital: "cd".repeat(32), validade: em(200) } },
  ] } }));

  await entrar(page);
  await expect(page.getByText("Certificados vencendo").locator("..")).toContainText("1");
  await page.getByRole("link", { name: "Contas", exact: true }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("alert")).toContainText("Um certificado precisa de atenção");
  await expect(page.getByRole("alert")).toContainText("Loja Efí");
  await expect(page.getByRole("row").filter({ hasText: "Loja Efí" })).toContainText("Vence em 12 dias");
  await expect(page.getByRole("row").filter({ hasText: "Loja BB" })).not.toContainText("Vence em");
});

test("sem o escopo, a tela explica qual falta", async ({ page }) => {
  // A key do sandbox tem todos os escopos: o 403 de uma key restrita é simulado.
  await page.route("**/vintem/v1/contas", (r) => r.fulfill({ status: 403, json: { type: "https://vintem.dev/problemas/SemEscopo", title: "Escopo insuficiente", status: 403 } }));
  await entrar(page);
  await page.goto("/painel/#/contas");
  await expect(page.getByRole("alert")).toContainText("conta:admin");
});
