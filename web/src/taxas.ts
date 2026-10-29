// Taxas públicas dos gateways, as mesmas do README ("Quanto você paga").
// Atualize as duas juntas, com a data da consulta.
export const consultadoEm = "29/10/2026";
export const gateways = [
  { nome: "Stripe (Pix, por convite)", fonte: "https://stripe.com/br/pricing", taxa: (c: number) => Math.round(c * 0.0119) },
  { nome: "Mercado Pago (checkout)", fonte: "https://www.mercadopago.com.br/ferramentas-para-vender/check-out", taxa: (c: number) => Math.round(c * 0.0099) },
  { nome: "Asaas (R$ 2,15 por Pix)", fonte: "https://www.asaas.com/precos-e-taxas", taxa: () => 215 },
];
