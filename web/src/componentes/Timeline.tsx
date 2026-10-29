import { StatusBadge } from "./StatusBadge";

export function Timeline({ eventos }: { eventos: { quando: string; status: string; detalhe?: string }[] }) {
  return (
    <ol style={{ listStyle: "none", margin: 0, padding: 0, display: "grid", gap: "var(--space-3)", borderLeft: "2px solid var(--border)", paddingLeft: "var(--space-4)" }}>
      {eventos.map((e, i) => (
        <li key={i}>
          <StatusBadge status={e.status} /> <time className="muted" dateTime={e.quando}>{new Date(e.quando).toLocaleString("pt-BR")}</time>
          {e.detalhe && <div className="muted">{e.detalhe}</div>}
        </li>
      ))}
    </ol>
  );
}
