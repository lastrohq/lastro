import { useEffect, useState } from "react";
import { contas, diasAte, semEscopo, type Conta } from "../api";
import { Card } from "../componentes/Card";
import { CopyField } from "../componentes/CopyField";
import { Table } from "../componentes/Table";

/** Certificado que vence em até 30 dias pede atenção, como o aviso certificado.expirando. */
export const DIAS_ALERTA = 30;

export function ValidadeCertificado({ conta }: { conta: Conta }) {
  if (!conta.certificado) return <span className="muted">sem certificado</span>;
  const dias = diasAte(conta.certificado.validade);
  const data = new Date(conta.certificado.validade).toLocaleDateString("pt-BR");
  if (dias <= 0) return <strong style={{ color: "var(--danger)" }}><span aria-hidden="true">⚠ </span>Venceu em {data}</strong>;
  if (dias <= DIAS_ALERTA) return <strong style={{ color: "var(--danger)" }}><span aria-hidden="true">⚠ </span>Vence em {dias} {dias === 1 ? "dia" : "dias"} ({data})</strong>;
  return <span>{data}</span>;
}

export function Contas() {
  const [lista, setLista] = useState<Conta[] | null>(null);
  const [erro, setErro] = useState("");
  useEffect(() => {
    contas().then(setLista, (e: Error) => setErro(semEscopo(e, "conta:admin") ?? e.message));
  }, []);

  const vencendo = (lista ?? []).filter((c) => c.certificado && diasAte(c.certificado.validade) <= DIAS_ALERTA);
  return (
    <>
      <h1>Contas</h1>
      {erro && <p role="alert" style={{ color: "var(--danger)" }}>{erro}</p>}
      {!lista && !erro && <p aria-busy="true" className="muted">Carregando…</p>}
      {vencendo.length > 0 && (
        <div role="alert" style={{ border: "1px solid var(--danger)", borderRadius: "var(--radius-lg)", padding: "var(--space-4)", marginBottom: "var(--space-4)", color: "var(--danger)" }}>
          <strong>⚠ {vencendo.length === 1 ? "Um certificado precisa" : `${vencendo.length} certificados precisam`} de atenção:</strong>{" "}
          {vencendo.map((c) => c.nome).join(", ")}. Sem certificado válido, o banco recusa as cobranças. Gere um novo no banco e atualize a conta.
        </div>
      )}
      {lista && (
        <Card>
          <Table
            rotulo="Contas recebedoras"
            linhas={[...lista].sort((a, b) => (a.certificado?.validade ?? "9").localeCompare(b.certificado?.validade ?? "9"))}
            chaveDa={(c) => c.id}
            colunas={[
              { chave: "nome", titulo: "Conta", render: (c) => <><div>{c.nome}</div><code className="mono muted">{c.id}</code></>, ordenar: (c) => c.nome },
              { chave: "psp", titulo: "Banco", render: (c) => `${c.psp} · ${c.ambiente}`, ordenar: (c) => c.psp },
              { chave: "chave", titulo: "Chave Pix", render: (c) => <CopyField valor={c.chave} rotulo={`chave Pix de ${c.nome}`} /> },
              { chave: "ativa", titulo: "Situação", render: (c) => (c.ativa ? "Ativa" : "Desativada") },
              { chave: "cert", titulo: "Certificado vale até", render: (c) => <ValidadeCertificado conta={c} />, ordenar: (c) => c.certificado?.validade ?? "9" },
            ]}
          />
        </Card>
      )}
    </>
  );
}
