const formato = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

/** Único jeito de mostrar dinheiro: recebe centavos. */
export function formatarCentavos(centavos: number): string {
  return formato.format(centavos / 100);
}

/** Converte o "49.90" da API Pix em centavos, sem passar por float. */
export function centavosDe(valor: string): number {
  const [reais = "0", cents = ""] = valor.split(".");
  return Number(reais) * 100 + Number((cents + "00").slice(0, 2));
}

export function Money({ centavos }: { centavos: number }) {
  return <span style={{ fontVariantNumeric: "tabular-nums" }}>{formatarCentavos(centavos)}</span>;
}
