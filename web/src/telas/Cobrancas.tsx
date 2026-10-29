import { useEffect, useState } from "react";
import { api, listar, type Cob } from "../api";
import { Card, EmptyState } from "../componentes/Card";
import { CopyField } from "../componentes/CopyField";
import { centavosDe, Money } from "../componentes/Money";
import { QRCode } from "../componentes/QRCode";
import { StatusBadge } from "../componentes/StatusBadge";
import { Table } from "../componentes/Table";
import { Timeline } from "../componentes/Timeline";

const filtros = [
  { valor: "", rotulo: "Todas" },
  { valor: "ATIVA", rotulo: "Ativas" },
  { valor: "CONCLUIDA", rotulo: "Pagas" },
  { valor: "REMOVIDA_PELO_USUARIO_RECEBEDOR", rotulo: "Canceladas" },
];

export function Cobrancas({ ir }: { ir: (rota: string) => void }) {
  const [status, setStatus] = useState("");
  const [cobs, setCobs] = useState<Cob[] | null>(null);
  const [erro, setErro] = useState("");
  useEffect(() => {
    setCobs(null);
    const agora = new Date();
    listar("cob", new Date(agora.getTime() - 90 * 86_400_000), agora, status || undefined)
      .then((l) => setCobs(l.sort((a, b) => b.calendario.criacao.localeCompare(a.calendario.criacao))))
      .catch((e: Error) => setErro(e.message));
  }, [status]);

  return (
    <>
      <h1>Cobranças</h1>
      <fieldset style={{ border: 0, padding: 0, margin: "0 0 var(--space-4)", display: "flex", gap: "var(--space-2)" }}>
        <legend className="sr-only">Filtrar por status</legend>
        {filtros.map((f) => (
          <label key={f.valor} style={{ display: "inline-flex", gap: "var(--space-1)", alignItems: "center", cursor: "pointer" }}>
            <input type="radio" name="status" value={f.valor} checked={status === f.valor} onChange={() => setStatus(f.valor)} />
            {f.rotulo}
          </label>
        ))}
      </fieldset>
      {erro && <p role="alert" style={{ color: "var(--danger)" }}>{erro}</p>}
      {!cobs && !erro && <p aria-busy="true" className="muted">Carregando…</p>}
      {cobs && cobs.length === 0 && <EmptyState titulo="Nenhuma cobrança nos últimos 90 dias" acao={<>Crie uma com <code className="mono">POST /cob</code>, o SDK ou o MCP.</>} />}
      {cobs && cobs.length > 0 && (
        <Table
          rotulo="Cobranças dos últimos 90 dias"
          linhas={cobs}
          chaveDa={(c) => c.txid}
          aoAbrir={(c) => ir(`/cobrancas/${c.txid}`)}
          colunas={[
            { chave: "criacao", titulo: "Criada em", render: (c) => new Date(c.calendario.criacao).toLocaleString("pt-BR"), ordenar: (c) => c.calendario.criacao },
            { chave: "valor", titulo: "Valor", render: (c) => <Money centavos={centavosDe(c.valor.original)} />, ordenar: (c) => centavosDe(c.valor.original) },
            { chave: "status", titulo: "Status", render: (c) => <StatusBadge status={c.status} />, ordenar: (c) => c.status },
            { chave: "txid", titulo: "txid", render: (c) => <code className="mono">{c.txid}</code> },
          ]}
        />
      )}
    </>
  );
}

export function CobrancaDetalhe({ txid, ir }: { txid: string; ir: (rota: string) => void }) {
  const [cob, setCob] = useState<Cob | null>(null);
  const [erro, setErro] = useState("");
  useEffect(() => {
    api<Cob>(`/cob/${encodeURIComponent(txid)}`).then(setCob, (e: Error) => setErro(e.message));
  }, [txid]);

  if (erro) return <p role="alert" style={{ color: "var(--danger)" }}>{erro}</p>;
  if (!cob) return <p aria-busy="true" className="muted">Carregando…</p>;
  const eventos = [
    { quando: cob.calendario.criacao, status: "ATIVA", detalhe: "Cobrança criada" },
    ...(cob.pix ?? []).map((p) => ({ quando: p.horario, status: "CONCLUIDA", detalhe: `Pix de ${p.valor} recebido (${p.endToEndId})` })),
  ];
  return (
    <>
      <p><a href="#/cobrancas" onClick={(e) => { e.preventDefault(); ir("/cobrancas"); }}>← Cobranças</a></p>
      <h1>
        <Money centavos={centavosDe(cob.valor.original)} /> <StatusBadge status={cob.status} />
      </h1>
      <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 2fr) minmax(0, 1fr)", gap: "var(--space-6)", alignItems: "start" }}>
        <div style={{ display: "grid", gap: "var(--space-4)" }}>
          <Card titulo="Dados">
            <dl style={{ display: "grid", gridTemplateColumns: "auto minmax(0, 1fr)", gap: "var(--space-2) var(--space-4)", margin: 0 }}>
              <dt className="muted">txid</dt><dd style={{ margin: 0 }}><CopyField valor={cob.txid} rotulo="txid" /></dd>
              <dt className="muted">Chave Pix</dt><dd style={{ margin: 0 }}><CopyField valor={cob.chave} rotulo="chave Pix" /></dd>
              {cob.solicitacaoPagador && (<><dt className="muted">Descrição</dt><dd style={{ margin: 0 }}>{cob.solicitacaoPagador}</dd></>)}
              <dt className="muted">Revisão</dt><dd style={{ margin: 0 }}>{cob.revisao}</dd>
            </dl>
          </Card>
          <Card titulo="Histórico">
            <Timeline eventos={eventos} />
          </Card>
        </div>
        {cob.pixCopiaECola && cob.status === "ATIVA" && (
          <Card titulo="Pagar">
            <QRCode brcode={cob.pixCopiaECola} />
          </Card>
        )}
      </div>
    </>
  );
}
