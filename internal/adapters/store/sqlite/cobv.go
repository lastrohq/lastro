package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/store/sqlite/db"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

type cobvs struct{ q *db.Queries }

// detalhesCobv é o JSON da coluna detalhes. Datas vão como AAAA-MM-DD.
type detalhesCobv struct {
	Multa      *domain.ValorModalidade `json:"multa,omitempty"`
	Juros      *domain.ValorModalidade `json:"juros,omitempty"`
	Abatimento *domain.ValorModalidade `json:"abatimento,omitempty"`
	Desconto   *descontoJSON           `json:"desconto,omitempty"`
	Email      string                  `json:"email,omitempty"`
	Logradouro string                  `json:"logradouro,omitempty"`
	Cidade     string                  `json:"cidade,omitempty"`
	UF         string                  `json:"uf,omitempty"`
	CEP        string                  `json:"cep,omitempty"`
}

type descontoJSON struct {
	Modalidade int                `json:"modalidade"`
	DatasFixas []descontoDataJSON `json:"datasFixas,omitempty"`
	ValorPerc  domain.Money       `json:"valorPerc,omitempty"`
}

type descontoDataJSON struct {
	Data      string       `json:"data"`
	ValorPerc domain.Money `json:"valorPerc"`
}

func paraDetalhes(c domain.CobrancaVencimento) (string, error) {
	d := detalhesCobv{Multa: c.Multa, Juros: c.Juros, Abatimento: c.Abatimento,
		Email: c.Devedor.Email, Logradouro: c.Devedor.Logradouro, Cidade: c.Devedor.Cidade, UF: c.Devedor.UF, CEP: c.Devedor.CEP}
	if c.Desconto != nil {
		d.Desconto = &descontoJSON{Modalidade: c.Desconto.Modalidade, ValorPerc: c.Desconto.ValorPerc}
		for _, df := range c.Desconto.DatasFixas {
			d.Desconto.DatasFixas = append(d.Desconto.DatasFixas, descontoDataJSON{Data: df.Data.String(), ValorPerc: df.ValorPerc})
		}
	}
	b, err := json.Marshal(d)
	return string(b), err
}

func paraCobv(r db.CobrancasVencimento) (domain.CobrancaVencimento, error) {
	venc, err := domain.ParseData(r.Vencimento)
	if err != nil {
		return domain.CobrancaVencimento{}, fmt.Errorf("cobv %s: vencimento corrompido: %w", r.Txid, err)
	}
	var d detalhesCobv
	if err := json.Unmarshal([]byte(r.Detalhes), &d); err != nil {
		return domain.CobrancaVencimento{}, fmt.Errorf("cobv %s: detalhes corrompidos: %w", r.Txid, err)
	}
	c := domain.CobrancaVencimento{
		Txid: domain.Txid(r.Txid), ContaID: r.ContaID, Chave: r.Chave, Status: domain.StatusCobranca(r.Status), Valor: domain.Money(r.Valor),
		Multa: d.Multa, Juros: d.Juros, Abatimento: d.Abatimento, Vencimento: venc, ValidadeAposVencimento: int(r.ValidadeDias),
		Devedor: domain.DevedorCompleto{
			Devedor: domain.Devedor{Documento: r.DevedorDocumento, Nome: r.DevedorNome},
			Email:   d.Email, Logradouro: d.Logradouro, Cidade: d.Cidade, UF: d.UF, CEP: d.CEP,
		},
		SolicitacaoPagador: r.SolicitacaoPagador, Location: r.Location, BRCode: r.BrCode,
		CriadaEm: time.Unix(0, r.CriadaEm).UTC(), Revisao: int(r.Revisao),
	}
	if d.Desconto != nil {
		c.Desconto = &domain.Desconto{Modalidade: d.Desconto.Modalidade, ValorPerc: d.Desconto.ValorPerc}
		for _, df := range d.Desconto.DatasFixas {
			data, err := domain.ParseData(df.Data)
			if err != nil {
				return domain.CobrancaVencimento{}, fmt.Errorf("cobv %s: data de desconto corrompida: %w", r.Txid, err)
			}
			c.Desconto.DatasFixas = append(c.Desconto.DatasFixas, domain.DescontoData{Data: data, ValorPerc: df.ValorPerc})
		}
	}
	return c, nil
}

func (r cobvs) Criar(ctx context.Context, c domain.CobrancaVencimento) error {
	det, err := paraDetalhes(c)
	if err != nil {
		return fmt.Errorf("cobv %s: %w", c.Txid, err)
	}
	err = r.q.CriarCobv(ctx, db.CriarCobvParams{
		ContaID: c.ContaID, Txid: string(c.Txid), Chave: c.Chave, Status: string(c.Status), Valor: int64(c.Valor),
		Vencimento: c.Vencimento.String(), ValidadeDias: int64(c.ValidadeAposVencimento),
		DevedorDocumento: c.Devedor.Documento, DevedorNome: c.Devedor.Nome, Detalhes: det,
		SolicitacaoPagador: c.SolicitacaoPagador, Location: c.Location, BrCode: c.BRCode, CriadaEm: c.CriadaEm.UnixNano(),
	})
	if chaveEstrangeira(err) {
		return fmt.Errorf("conta %s: %w", c.ContaID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, "cobv "+string(c.Txid))
}

func (r cobvs) BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.CobrancaVencimento, error) {
	row, err := r.q.BuscarCobvPorTxid(ctx, string(txid))
	if err != nil {
		return domain.CobrancaVencimento{}, traduzir(err, "cobv "+string(txid))
	}
	return paraCobv(row)
}

func (r cobvs) Atualizar(ctx context.Context, c *domain.CobrancaVencimento) error {
	det, err := paraDetalhes(*c)
	if err != nil {
		return fmt.Errorf("cobv %s: %w", c.Txid, err)
	}
	n, err := r.q.AtualizarCobv(ctx, db.AtualizarCobvParams{
		Status: string(c.Status), Valor: int64(c.Valor), Vencimento: c.Vencimento.String(), ValidadeDias: int64(c.ValidadeAposVencimento),
		DevedorDocumento: c.Devedor.Documento, DevedorNome: c.Devedor.Nome, Detalhes: det,
		SolicitacaoPagador: c.SolicitacaoPagador, Location: c.Location, BrCode: c.BRCode,
		Txid: string(c.Txid), Revisao: int64(c.Revisao),
	})
	if err != nil {
		return traduzir(err, "cobv "+string(c.Txid))
	}
	if n == 0 {
		if _, err := r.BuscarPorTxid(ctx, c.Txid); err != nil {
			return err
		}
		return fmt.Errorf("cobv %s, revisão %d: %w", c.Txid, c.Revisao, ports.ErrConflito)
	}
	c.Revisao++
	return nil
}

func (r cobvs) Listar(ctx context.Context, f ports.FiltroCobrancas) ([]domain.CobrancaVencimento, string, error) {
	var depoisDe int64
	if f.Cursor != "" {
		n, err := strconv.ParseInt(f.Cursor, 10, 64)
		if err != nil || n < 0 {
			return nil, "", fmt.Errorf("cursor inválido %q", f.Cursor)
		}
		depoisDe = n
	}
	limite := f.Limite
	if limite <= 0 {
		limite = 100
	}
	p := filtros(f)
	rows, err := r.q.ListarCobvs(ctx, db.ListarCobvsParams{
		DepoisDe: depoisDe, ContaID: p.ContaID, Status: p.Status, Documento: p.Documento, Inicio: p.Inicio, Fim: p.Fim,
		Pular: int64(f.Pular), Limite: int64(limite) + 1,
	})
	if err != nil {
		return nil, "", traduzir(err, "cobvs")
	}
	prox := ""
	if len(rows) > limite {
		rows = rows[:limite]
		prox = strconv.FormatInt(rows[limite-1].Seq, 10)
	}
	out := make([]domain.CobrancaVencimento, len(rows))
	for i, row := range rows {
		if out[i], err = paraCobv(row); err != nil {
			return nil, "", err
		}
	}
	return out, prox, nil
}

func (r cobvs) Contar(ctx context.Context, f ports.FiltroCobrancas) (int, error) {
	p := filtros(f)
	n, err := r.q.ContarCobvs(ctx, db.ContarCobvsParams{ContaID: p.ContaID, Status: p.Status, Documento: p.Documento, Inicio: p.Inicio, Fim: p.Fim})
	if err != nil {
		return 0, traduzir(err, "cobvs")
	}
	return int(n), nil
}
