import { useCallback, useEffect, useState } from "react";
import { entregas, reenviar, semEscopo, type Entrega } from "../api";
import { Button } from "../componentes/Button";
import { EmptyState } from "../componentes/Card";
import { StatusBadge } from "../componentes/StatusBadge";
import { Table } from "../componentes/Table";

const abas: { status: Entrega["status"]; rotulo: string }[] = [
  { status: "dlq", rotulo: "Na DLQ" },
  { status: "pendente", rotulo: "Pendentes" },
  { status: "entregue", rotulo: "Entregues" },
];
const data = (iso: string) => new Date(iso).toLocaleString("pt-BR");

export function Webhooks() {
  const [status, setStatus] = useState<Entrega["status"]>("dlq");
  const [lista, setLista] = useState<Entrega[] | null>(null);
  const [erro, setErro] = useState("");
  const [aviso, setAviso] = useState("");
  const [reenviando, setReenviando] = useState("");

  const carregar = useCallback(() => {
    setLista(null);
    entregas(status).then(setLista, (e: Error) => setErro(semEscopo(e, "webhook:admin") ?? e.message));
  }, [status]);
  useEffect(carregar, [carregar]);

  return (
    <>
      <h1>Webhooks</h1>
      <p className="muted" style={{ marginTop: 0 }}>Entregas dos eventos à sua aplicação. Depois de 24 h de falhas, a entrega vai para a DLQ e só sai dela por reenvio.</p>
      <fieldset style={{ border: 0, padding: 0, margin: "0 0 var(--space-4)", display: "flex", gap: "var(--space-2)" }}>
        <legend className="sr-only">Filtrar entregas</legend>
        {abas.map((a) => (
          <label key={a.status} style={{ display: "inline-flex", gap: "var(--space-1)", alignItems: "center", cursor: "pointer" }}>
            <input type="radio" name="entregas" checked={status === a.status} onChange={() => { setAviso(""); setStatus(a.status); }} />
            {a.rotulo}
          </label>
        ))}
      </fieldset>
      <p role="status" aria-live="polite" style={{ color: "var(--success)", minHeight: "1.5em", margin: "0 0 var(--space-2)" }}>{aviso}</p>
      {erro && <p role="alert" style={{ color: "var(--danger)" }}>{erro}</p>}
      {!lista && !erro && <p aria-busy="true" className="muted">Carregando…</p>}
      {lista && lista.length === 0 && (
        <EmptyState titulo={status === "dlq" ? "Nenhuma entrega na DLQ" : "Nenhuma entrega aqui"} acao={status === "dlq" ? "Tudo o que o Lastro tentou entregar chegou." : <>Cadastre um destino com <code className="mono">lastro destino criar --url ...</code></>} />
      )}
      {lista && lista.length > 0 && (
        <Table
          rotulo={`Entregas: ${abas.find((a) => a.status === status)!.rotulo}`}
          linhas={lista}
          chaveDa={(e) => `${e.eventoId}/${e.destinoId}`}
          colunas={[
            { chave: "tipo", titulo: "Evento", render: (e) => <><div>{e.tipo}</div><code className="mono muted">{e.eventoId}</code></>, ordenar: (e) => e.tipo },
            { chave: "conta", titulo: "Conta", render: (e) => e.contaId, ordenar: (e) => e.contaId },
            { chave: "status", titulo: "Status", render: (e) => <StatusBadge status={e.status} /> },
            { chave: "tentativas", titulo: "Tentativas", render: (e) => e.tentativas, ordenar: (e) => e.tentativas },
            { chave: "ultimo", titulo: "Último resultado", render: (e) => (e.ultimoHttp ? `HTTP ${e.ultimoHttp}` : e.ultimoErro ? "" : "—") + (e.ultimoErro ? ` ${e.ultimoErro}` : "") },
            { chave: "quando", titulo: status === "pendente" ? "Próxima tentativa" : "Atualizada em", render: (e) => data(status === "pendente" ? e.proximaEm : e.atualizadaEm), ordenar: (e) => e.atualizadaEm },
            ...(status === "dlq"
              ? [{
                  chave: "acao",
                  titulo: "Ação",
                  render: (e: Entrega) => (
                    <Button
                      variante="secondary"
                      carregando={reenviando === e.eventoId}
                      aria-label={`Reenviar ${e.tipo} ${e.eventoId}`}
                      onClick={async () => {
                        setReenviando(e.eventoId);
                        try {
                          const r = await reenviar(e.eventoId);
                          setAviso(`${r.reenviadas} ${r.reenviadas === 1 ? "entrega voltou" : "entregas voltaram"} para a fila; a nova tentativa sai em instantes.`);
                          carregar();
                        } catch (err) {
                          setErro((err as Error).message);
                        } finally {
                          setReenviando("");
                        }
                      }}
                    >
                      Reenviar
                    </Button>
                  ),
                }]
              : []),
          ]}
        />
      )}
    </>
  );
}
