import type { ReactNode } from "react";

export function Card({ children, titulo }: { children: ReactNode; titulo?: string }) {
  return (
    <section style={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", padding: "var(--space-6)" }}>
      {titulo && <h2>{titulo}</h2>}
      {children}
    </section>
  );
}

export function Stat({ rotulo, valor, detalhe }: { rotulo: string; valor: ReactNode; detalhe?: ReactNode }) {
  return (
    <Card>
      <div className="muted">{rotulo}</div>
      <div style={{ fontSize: "var(--text-32)", fontWeight: 700, fontVariantNumeric: "tabular-nums" }}>{valor}</div>
      {detalhe && <div className="muted" style={{ fontSize: "var(--text-12)" }}>{detalhe}</div>}
    </Card>
  );
}

export function EmptyState({ titulo, acao }: { titulo: string; acao: ReactNode }) {
  return (
    <div style={{ textAlign: "center", padding: "var(--space-12) var(--space-6)", border: "1px dashed var(--border)", borderRadius: "var(--radius-lg)" }}>
      <p style={{ margin: "0 0 var(--space-2)", fontWeight: 600 }}>{titulo}</p>
      <div className="muted">{acao}</div>
    </div>
  );
}
