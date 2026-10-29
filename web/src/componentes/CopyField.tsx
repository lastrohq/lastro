import { useState } from "react";

export function CopyField({ valor, rotulo }: { valor: string; rotulo: string }) {
  const [copiado, setCopiado] = useState(false);
  return (
    <div style={{ display: "flex", gap: "var(--space-2)", alignItems: "center", minWidth: 0 }}>
      <code className="mono" title={valor} style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", minWidth: 0 }}>{valor}</code>
      <button
        type="button"
        aria-label={`Copiar ${rotulo}`}
        onClick={async () => {
          await navigator.clipboard?.writeText(valor);
          setCopiado(true);
          setTimeout(() => setCopiado(false), 2000);
        }}
        style={{ background: "transparent", border: "1px solid var(--border)", borderRadius: "var(--radius-sm)", color: "var(--text)", cursor: "pointer", padding: "0 var(--space-2)", font: "inherit", fontSize: "var(--text-12)" }}
      >
        {copiado ? "Copiado" : "Copiar"}
      </button>
      <span className="sr-only" aria-live="polite">{copiado ? `${rotulo} copiado` : ""}</span>
    </div>
  );
}
