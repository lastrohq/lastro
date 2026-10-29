import { useEffect, useState } from "react";
import { contas, diasAte, entregas, listar, type Cob } from "../api";
import { Card, EmptyState, Stat } from "../componentes/Card";
import { centavosDe, Money } from "../componentes/Money";
import { consultadoEm, gateways } from "../taxas";

function inicioDoDia(d: Date) { return new Date(d.getFullYear(), d.getMonth(), d.getDate()); }
function inicioDoMes(d: Date) { return new Date(d.getFullYear(), d.getMonth(), 1); }

export function VisaoGeral({ ir }: { ir: (rota: string) => void }) {
  const [dados, setDados] = useState<{ cobs: Cob[]; ativas: number } | null>(null);
  const [erro, setErro] = useState("");
  // Cada card só aparece se a API key tiver o escopo dele (null = sem acesso).
  const [dlq, setDlq] = useState<number | null>(null);
  const [vencendo, setVencendo] = useState<number | null>(null);
  useEffect(() => {
    entregas("dlq").then((l) => setDlq(l.length), () => setDlq(null));
    contas().then((l) => setVencendo(l.filter((c) => c.certificado && diasAte(c.certificado.validade) <= 30).length), () => setVencendo(null));
  }, []);
  useEffect(() => {
    const agora = new Date();
    // Pix pago no mês pode ser de cobrança criada antes: olha 90 dias para trás.
    const desde = new Date(agora.getTime() - 90 * 86_400_000);
    Promise.all([listar("cob", desde, agora), listar("cobv", desde, agora).catch(() => [])])
      .then(([cob, cobv]) => {
        const cobs = [...cob, ...cobv];
        setDados({ cobs, ativas: cobs.filter((c) => c.status === "ATIVA").length });
      })
      .catch((e: Error) => setErro(e.message));
  }, []);

  if (erro) return <p role="alert" style={{ color: "var(--danger)" }}>{erro}</p>;
  if (!dados) return <p aria-busy="true" className="muted">Carregando…</p>;

  const agora = new Date();
  const pix = dados.cobs.flatMap((c) => c.pix ?? []);
  const soma = (desde: Date) => pix.filter((p) => new Date(p.horario) >= desde).reduce((t, p) => t + centavosDe(p.valor), 0);
  const doMes = pix.filter((p) => new Date(p.horario) >= inicioDoMes(agora));
  const recebidoMes = soma(inicioDoMes(agora));

  return (
    <>
      <h1>Visão geral</h1>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: "var(--space-4)", marginBottom: "var(--space-6)" }}>
        <Stat rotulo="Recebido hoje" valor={<Money centavos={soma(inicioDoDia(agora))} />} />
        <Stat rotulo="Recebido no mês" valor={<Money centavos={recebidoMes} />} detalhe={`${doMes.length} Pix`} />
        <Stat rotulo="Cobranças ativas" valor={dados.ativas} detalhe="aguardando pagamento" />
        {dlq !== null && (
          <Stat rotulo="Webhooks na DLQ" valor={<span style={{ color: dlq > 0 ? "var(--danger)" : undefined }}>{dlq}</span>}
            detalhe={dlq > 0 ? <a href="#/webhooks" onClick={(e) => { e.preventDefault(); ir("/webhooks"); }}>ver e reenviar</a> : "nenhum evento perdido"} />
        )}
        {vencendo !== null && (
          <Stat rotulo="Certificados vencendo" valor={<span style={{ color: vencendo > 0 ? "var(--danger)" : undefined }}>{vencendo}</span>}
            detalhe={vencendo > 0 ? <a href="#/contas" onClick={(e) => { e.preventDefault(); ir("/contas"); }}>ver contas</a> : "nos próximos 30 dias"} />
        )}
      </div>
      {pix.length === 0 ? (
        <EmptyState titulo="Nenhum Pix recebido ainda" acao={<>Crie sua primeira cobrança no sandbox: <code className="mono">lastro sandbox</code></>} />
      ) : (
        <Card titulo="Quanto você pagaria num gateway">
          <p className="muted" style={{ marginTop: 0 }}>
            Pelos {doMes.length} Pix deste mês (<Money centavos={recebidoMes} />). O Lastro não cobra nada por Pix; a sua economia é o valor abaixo
            menos a tarifa de Pix do seu banco.
          </p>
          <ul style={{ margin: 0, paddingLeft: "var(--space-4)" }}>
            {gateways.map((g) => (
              <li key={g.nome}>
                {g.nome}: <strong><Money centavos={doMes.reduce((t, p) => t + g.taxa(centavosDe(p.valor)), 0)} /></strong> <a href={g.fonte} target="_blank" rel="noreferrer">fonte</a>
              </li>
            ))}
          </ul>
          <p className="muted" style={{ marginBottom: 0, fontSize: "var(--text-12)" }}>Taxas das páginas públicas de preço, consultadas em {consultadoEm}.</p>
        </Card>
      )}
    </>
  );
}
