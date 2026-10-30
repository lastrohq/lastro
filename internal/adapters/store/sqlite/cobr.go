package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/store/sqlite/db"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// CobrancasRecorrentes devolve o repositório de cobranças recorrentes.
func (s *Store) CobrancasRecorrentes() ports.CobrancaRecorrenteRepo { return cobrs{s.q} }

type cobrs struct{ q *db.Queries }

// tentativaJSON é uma tentativa na coluna tentativas. Datas vão como AAAA-MM-DD.
type tentativaJSON struct {
	DataLiquidacao string `json:"dataLiquidacao"`
	Tipo           string `json:"tipo"`
	Status         string `json:"status"`
	EndToEndID     string `json:"endToEndId,omitempty"`
	MotivoRejeicao string `json:"motivoRejeicao,omitempty"`
}

func tentativasJSON(ts []domain.Tentativa) (string, error) {
	out := make([]tentativaJSON, len(ts))
	for i, t := range ts {
		out[i] = tentativaJSON{
			DataLiquidacao: t.DataLiquidacao.String(), Tipo: string(t.Tipo), Status: string(t.Status),
			EndToEndID: t.EndToEndID, MotivoRejeicao: t.MotivoRejeicao,
		}
	}
	b, err := json.Marshal(out)
	return string(b), err
}

func lerTentativas(s string) ([]domain.Tentativa, error) {
	var in []tentativaJSON
	if err := json.Unmarshal([]byte(s), &in); err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]domain.Tentativa, len(in))
	for i, t := range in {
		d, err := domain.ParseData(t.DataLiquidacao)
		if err != nil {
			return nil, err
		}
		out[i] = domain.Tentativa{
			DataLiquidacao: d, Tipo: domain.TipoTentativa(t.Tipo), Status: domain.StatusTentativa(t.Status),
			EndToEndID: t.EndToEndID, MotivoRejeicao: t.MotivoRejeicao,
		}
	}
	return out, nil
}

func (r cobrs) Criar(ctx context.Context, c domain.CobrancaRecorrente) error {
	tentativas, err := tentativasJSON(c.Tentativas)
	if err != nil {
		return fmt.Errorf("cobrança recorrente %s: tentativas: %w", c.Txid, err)
	}
	err = r.q.CriarCobr(ctx, db.CriarCobrParams{
		Txid: string(c.Txid), RecorrenciaID: c.RecorrenciaID, ContaID: c.ContaID, Status: string(c.Status),
		Valor: int64(c.Valor), Vencimento: c.Vencimento.String(), AjusteDiaUtil: boolInt(c.AjusteDiaUtil),
		InfoAdicional: c.InfoAdicional, Tentativas: tentativas, MotivoEncerramento: c.MotivoEncerramento,
		CriadaEm: c.CriadaEm.UnixNano(), Revisao: int64(c.Revisao),
	})
	if chaveEstrangeira(err) {
		return fmt.Errorf("recorrência %s: %w", c.RecorrenciaID, ports.ErrNaoEncontrado)
	}
	return traduzir(err, "cobrança "+string(c.Txid))
}

func paraCobr(row db.CobrancasRecorrente) (domain.CobrancaRecorrente, error) {
	venc, err := domain.ParseData(row.Vencimento)
	if err != nil {
		return domain.CobrancaRecorrente{}, fmt.Errorf("cobrança recorrente %s: vencimento: %w", row.Txid, err)
	}
	tentativas, err := lerTentativas(row.Tentativas)
	if err != nil {
		return domain.CobrancaRecorrente{}, fmt.Errorf("cobrança recorrente %s: tentativas: %w", row.Txid, err)
	}
	return domain.CobrancaRecorrente{
		Txid: domain.Txid(row.Txid), RecorrenciaID: row.RecorrenciaID, ContaID: row.ContaID,
		Status: domain.StatusCobrancaRecorrente(row.Status), Valor: domain.Money(row.Valor), Vencimento: venc,
		AjusteDiaUtil: row.AjusteDiaUtil == 1, InfoAdicional: row.InfoAdicional, Tentativas: tentativas,
		MotivoEncerramento: row.MotivoEncerramento, CriadaEm: time.Unix(0, row.CriadaEm).UTC(), Revisao: int(row.Revisao),
	}, nil
}

func (r cobrs) BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	row, err := r.q.BuscarCobr(ctx, string(txid))
	if err != nil {
		return domain.CobrancaRecorrente{}, traduzir(err, "cobrança recorrente "+string(txid))
	}
	return paraCobr(row)
}

func (r cobrs) Atualizar(ctx context.Context, c *domain.CobrancaRecorrente) error {
	tentativas, err := tentativasJSON(c.Tentativas)
	if err != nil {
		return fmt.Errorf("cobrança recorrente %s: tentativas: %w", c.Txid, err)
	}
	n, err := r.q.AtualizarCobr(ctx, db.AtualizarCobrParams{
		Status: string(c.Status), Tentativas: tentativas, MotivoEncerramento: c.MotivoEncerramento,
		Txid: string(c.Txid), Revisao: int64(c.Revisao),
	})
	if err != nil {
		return traduzir(err, "cobrança recorrente "+string(c.Txid))
	}
	if n == 0 {
		existe, err := r.q.ExisteCobr(ctx, string(c.Txid))
		if err != nil {
			return traduzir(err, "cobrança recorrente "+string(c.Txid))
		}
		if existe == 0 {
			return fmt.Errorf("cobrança recorrente %s: %w", c.Txid, ports.ErrNaoEncontrado)
		}
		return fmt.Errorf("cobrança recorrente %s fora da revisão %d: %w", c.Txid, c.Revisao, ports.ErrConflito)
	}
	c.Revisao++
	return nil
}

func (r cobrs) DaRecorrencia(ctx context.Context, idRec string) ([]domain.CobrancaRecorrente, error) {
	rows, err := r.q.CobrsDaRecorrencia(ctx, idRec)
	if err != nil {
		return nil, traduzir(err, "cobranças da recorrência "+idRec)
	}
	out := make([]domain.CobrancaRecorrente, len(rows))
	for i, row := range rows {
		if out[i], err = paraCobr(row); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r cobrs) Abertas(ctx context.Context, contaID string) ([]domain.CobrancaRecorrente, error) {
	rows, err := r.q.CobrsAbertas(ctx, contaID)
	if err != nil {
		return nil, traduzir(err, "cobranças recorrentes da conta "+contaID)
	}
	out := make([]domain.CobrancaRecorrente, len(rows))
	for i, row := range rows {
		if out[i], err = paraCobr(row); err != nil {
			return nil, err
		}
	}
	return out, nil
}
