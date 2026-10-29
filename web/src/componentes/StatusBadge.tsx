type Tom = "success" | "warning" | "danger" | "info" | "muted";

// Um por estado de domínio (internal/domain): ícone + texto, nunca só cor.
export const estados: Record<string, { rotulo: string; tom: Tom; icone: string }> = {
  // cob, cobv e cobr
  ATIVA: { rotulo: "Ativa", tom: "warning", icone: "◷" },
  CONCLUIDA: { rotulo: "Paga", tom: "success", icone: "✓" },
  REMOVIDA_PELO_USUARIO_RECEBEDOR: { rotulo: "Cancelada", tom: "danger", icone: "✕" },
  REMOVIDA_PELO_PSP: { rotulo: "Removida pelo banco", tom: "danger", icone: "✕" },
  // Pix Automático
  CRIADA: { rotulo: "Criada", tom: "info", icone: "○" },
  APROVADA: { rotulo: "Aprovada", tom: "success", icone: "✓" },
  REJEITADA: { rotulo: "Rejeitada", tom: "danger", icone: "✕" },
  EXPIRADA: { rotulo: "Expirada", tom: "danger", icone: "⌛" },
  CANCELADA: { rotulo: "Cancelada", tom: "danger", icone: "✕" },
  ENVIADA: { rotulo: "Enviada", tom: "info", icone: "↗" },
  RECEBIDA: { rotulo: "Recebida", tom: "info", icone: "↘" },
  ACEITA: { rotulo: "Aceita", tom: "success", icone: "✓" },
  AGENDADA: { rotulo: "Agendada", tom: "info", icone: "◷" },
  SOLICITADA: { rotulo: "Solicitada", tom: "warning", icone: "◷" },
  PAGA: { rotulo: "Paga", tom: "success", icone: "✓" },
  // devolução
  EM_PROCESSAMENTO: { rotulo: "Em processamento", tom: "warning", icone: "◷" },
  DEVOLVIDO: { rotulo: "Devolvido", tom: "success", icone: "↩" },
  NAO_REALIZADO: { rotulo: "Não realizada", tom: "danger", icone: "✕" },
  // entrega de webhook
  pendente: { rotulo: "Pendente", tom: "warning", icone: "◷" },
  entregue: { rotulo: "Entregue", tom: "success", icone: "✓" },
  dlq: { rotulo: "Na DLQ", tom: "danger", icone: "⚠" },
};

export function StatusBadge({ status }: { status: string }) {
  const e = estados[status] ?? { rotulo: status, tom: "muted" as Tom, icone: "•" };
  const cor = e.tom === "muted" ? "var(--text-muted)" : `var(--${e.tom})`;
  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: "var(--space-1)", color: cor, border: `1px solid ${cor}`, borderRadius: "999px", padding: "0 var(--space-2)", fontSize: "var(--text-12)", fontWeight: 600, whiteSpace: "nowrap" }}>
      <span aria-hidden="true">{e.icone}</span>
      {e.rotulo}
    </span>
  );
}
