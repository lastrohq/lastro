import { useMemo, useState, type ReactNode } from "react";

export interface Coluna<T> {
  chave: string;
  titulo: string;
  render: (linha: T) => ReactNode;
  ordenar?: (linha: T) => string | number;
}

/** Tabela ordenável; linha clicável (e acionável por Enter) abre o detalhe. */
export function Table<T>({ colunas, linhas, chaveDa, aoAbrir, rotulo }: { colunas: Coluna<T>[]; linhas: T[]; chaveDa: (l: T) => string; aoAbrir?: (l: T) => void; rotulo: string }) {
  const [ordem, setOrdem] = useState<{ chave: string; asc: boolean } | null>(null);
  const ordenadas = useMemo(() => {
    const col = colunas.find((c) => c.chave === ordem?.chave);
    if (!col?.ordenar) return linhas;
    return [...linhas].sort((a, b) => {
      const [x, y] = [col.ordenar!(a), col.ordenar!(b)];
      return (x < y ? -1 : x > y ? 1 : 0) * (ordem!.asc ? 1 : -1);
    });
  }, [linhas, colunas, ordem]);
  const celula: React.CSSProperties = { padding: "var(--space-3) var(--space-4)", borderBottom: "1px solid var(--border)", textAlign: "left" };
  return (
    <table aria-label={rotulo} style={{ width: "100%", borderCollapse: "collapse", background: "var(--surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
      <thead>
        <tr>
          {colunas.map((c) => (
            <th key={c.chave} scope="col" style={{ ...celula, color: "var(--text-muted)", fontWeight: 600 }} aria-sort={ordem?.chave === c.chave ? (ordem.asc ? "ascending" : "descending") : undefined}>
              {c.ordenar ? (
                <button type="button" onClick={() => setOrdem((o) => ({ chave: c.chave, asc: o?.chave === c.chave ? !o.asc : true }))} style={{ all: "unset", cursor: "pointer" }}>
                  {c.titulo} {ordem?.chave === c.chave ? (ordem.asc ? "↑" : "↓") : ""}
                </button>
              ) : (
                c.titulo
              )}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {ordenadas.map((l) => (
          <tr
            key={chaveDa(l)}
            tabIndex={aoAbrir ? 0 : undefined}
            onClick={aoAbrir ? () => aoAbrir(l) : undefined}
            onKeyDown={aoAbrir ? (e) => e.key === "Enter" && aoAbrir(l) : undefined}
            style={{ cursor: aoAbrir ? "pointer" : undefined }}
          >
            {colunas.map((c) => (
              <td key={c.chave} style={celula}>{c.render(l)}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
