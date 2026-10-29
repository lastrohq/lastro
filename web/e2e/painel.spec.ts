// Fluxo crítico do painel (#35), feito só pelo teclado.
import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

const token = () => process.env.LASTRO_TOKEN!;
async function criar(api: APIRequestContext, valor: string, descricao: string) {
  const r = await api.post("/cob", {
    headers: { Authorization: `Bearer ${token()}`, "Idempotency-Key": `e2e-${descricao}-${Date.now()}` },
    data: { calendario: { expiracao: 3600 }, valor: { original: valor }, chave: "sandbox@lastro.dev", solicitacaoPagador: descricao },
  });
  expect(r.status()).toBe(201);
  return (await r.json()).txid as string;
}
async function teclar(page: Page, alvo: ReturnType<Page["locator"]>, tecla = "Enter") {
  await alvo.focus();
  await page.keyboard.press(tecla);
}

test("entrar, ver o recebido, achar a cobrança paga e o QR da ativa", async ({ page, request }) => {
  const paga = await criar(request, "49.90", "pedido pago");
  // O sandbox paga em 5 s e a conciliação (2 s aqui) registra o Pix.
  await expect.poll(async () => (await (await request.get(`/cob/${paga}`, { headers: { Authorization: `Bearer ${token()}` } })).json()).pix?.length ?? 0, { timeout: 30_000 }).toBe(1);
  const ativa = await criar(request, "15.00", "pedido em aberto");

  await page.goto("/painel/");
  const campo = page.getByLabel("API key");
  await campo.focus();
  await page.keyboard.type("lst_errado");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("alert")).toContainText("inválida");

  await campo.fill("");
  await page.keyboard.type(token());
  await page.keyboard.press("Enter");

  await expect(page.getByRole("heading", { name: "Visão geral" })).toBeVisible();
  await expect(page.getByText("Recebido hoje").locator("..")).toContainText("R$ 49,90");
  await expect(page.getByText("Cobranças ativas").locator("..")).toContainText("1");
  await expect(page.getByText(/Mercado Pago \(checkout\): R\$\s0,49/)).toBeVisible();
  await expect(page.getByText(/consultadas em 29\/10\/2026/)).toBeVisible();

  await teclar(page, page.getByRole("link", { name: "Cobranças" }));
  await expect(page.getByRole("heading", { name: "Cobranças" })).toBeVisible();
  await page.getByLabel("Pagas").focus();
  await page.keyboard.press("Space");
  const linhaPaga = page.getByRole("row").filter({ hasText: paga });
  await expect(linhaPaga).toBeVisible();
  await expect(page.getByRole("row").filter({ hasText: ativa })).toHaveCount(0);
  await teclar(page, linhaPaga);

  await expect(page.getByRole("heading", { name: /R\$\s49,90/ })).toContainText("Paga");
  await expect(page.getByText(/Pix de 49.90 recebido/)).toBeVisible();

  await page.goto(`/painel/#/cobrancas/${ativa}`);
  await expect(page.getByRole("img", { name: "QR Code do Pix" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Copiar Pix Copia e Cola" })).toBeVisible();
});
