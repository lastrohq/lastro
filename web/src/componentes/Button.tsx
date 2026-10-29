import type { ButtonHTMLAttributes } from "react";

type Variante = "primary" | "secondary" | "ghost" | "danger";
const estilos: Record<Variante, React.CSSProperties> = {
  primary: { background: "var(--text)", color: "var(--bg)", border: "1px solid var(--text)" },
  secondary: { background: "var(--surface)", color: "var(--text)", border: "1px solid var(--border)" },
  ghost: { background: "transparent", color: "var(--text)", border: "1px solid transparent" },
  danger: { background: "transparent", color: "var(--danger)", border: "1px solid var(--danger)" },
};

/** Loading troca o texto por "…" sem mudar a largura: o texto fica, invisível. */
export function Button({ variante = "secondary", carregando = false, children, style, disabled, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variante?: Variante; carregando?: boolean }) {
  return (
    <button
      {...props}
      disabled={disabled || carregando}
      aria-busy={carregando || undefined}
      style={{ ...estilos[variante], position: "relative", borderRadius: "var(--radius-md)", padding: "var(--space-2) var(--space-4)", font: "inherit", fontWeight: 600, cursor: disabled || carregando ? "default" : "pointer", opacity: disabled ? 0.6 : 1, ...style }}
    >
      <span style={{ visibility: carregando ? "hidden" : "visible" }}>{children}</span>
      {carregando && <span style={{ position: "absolute", inset: 0, display: "grid", placeItems: "center" }}>…</span>}
    </button>
  );
}
