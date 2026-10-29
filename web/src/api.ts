// Cliente da API do Lastro para o painel: mesma origem, API key do usuário.
export interface Pix { endToEndId: string; txid?: string; valor: string; horario: string; infoPagador?: string }
export interface Cob {
  txid: string;
  status: string;
  valor: { original: string };
  calendario: { criacao: string; expiracao?: number; dataDeVencimento?: string };
  chave: string;
  pixCopiaECola?: string;
  solicitacaoPagador?: string;
  revisao: number;
  pix?: Pix[];
}

export class ErroAPI extends Error {
  constructor(readonly status: number, mensagem: string) {
    super(mensagem);
  }
}

const CHAVE = "lastro.painel.token";
export const sessao = {
  token: () => sessionStorage.getItem(CHAVE),
  entrar: (t: string) => sessionStorage.setItem(CHAVE, t),
  sair: () => sessionStorage.removeItem(CHAVE),
};

export async function api<T>(caminho: string, token = sessao.token()): Promise<T> {
  const resp = await fetch(caminho, { headers: { Authorization: `Bearer ${token ?? ""}` } });
  const texto = await resp.text();
  if (!resp.ok) {
    let msg = `o Lastro respondeu ${resp.status}`;
    try {
      const p = JSON.parse(texto) as { title?: string; detail?: string };
      msg = p.detail ?? p.title ?? msg;
    } catch {
      // corpo fora do formato
    }
    throw new ErroAPI(resp.status, msg);
  }
  return JSON.parse(texto) as T;
}

/** Lista cob ou cobv de um período, seguindo a paginação (até 10 páginas de 1000). */
export async function listar(tipo: "cob" | "cobv", inicio: Date, fim: Date, status?: string): Promise<Cob[]> {
  const todas: Cob[] = [];
  for (let pagina = 0; pagina < 10; pagina++) {
    const q = new URLSearchParams({ inicio: inicio.toISOString(), fim: fim.toISOString(), "paginacao.paginaAtual": String(pagina), "paginacao.itensPorPagina": "1000" });
    if (status) q.set("status", status);
    const r = await api<{ cobs: Cob[]; parametros: { paginacao: { quantidadeDePaginas: number } } }>(`/${tipo}?${q}`);
    todas.push(...r.cobs);
    if (pagina + 1 >= r.parametros.paginacao.quantidadeDePaginas) break;
  }
  return todas;
}
