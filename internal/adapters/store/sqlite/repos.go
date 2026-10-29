package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/store/sqlite/db"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

type contas struct{ q *db.Queries }

func (r contas) Criar(ctx context.Context, c domain.Conta) error {
	p := db.CriarContaParams{
		ID: c.ID, Nome: c.Nome, Psp: c.PSP, ChavePix: c.ChavePix, Ambiente: string(c.Ambiente), Ativa: boolInt(c.Ativa),
	}
	if cert := c.Certificado; cert != nil {
		p.CertSujeito = sql.NullString{String: cert.Sujeito, Valid: true}
		p.CertImpressaoDigital = sql.NullString{String: cert.ImpressaoDigital, Valid: true}
		p.CertValidadeS = sql.NullInt64{Int64: cert.Validade.Unix(), Valid: true}
	}
	return traduzir(r.q.CriarConta(ctx, p), "conta "+c.ID)
}

func (r contas) Buscar(ctx context.Context, id string) (domain.Conta, error) {
	row, err := r.q.BuscarConta(ctx, id)
	if err != nil {
		return domain.Conta{}, traduzir(err, "conta "+id)
	}
	return paraConta(row), nil
}

func (r contas) BuscarPorChave(ctx context.Context, chave string) (domain.Conta, error) {
	row, err := r.q.BuscarContaPorChave(ctx, chave)
	if err != nil {
		return domain.Conta{}, traduzir(err, "conta da chave "+chave)
	}
	return paraConta(db.BuscarContaRow(row)), nil
}

func (r contas) Listar(ctx context.Context) ([]domain.Conta, error) {
	rows, err := r.q.ListarContas(ctx)
	if err != nil {
		return nil, traduzir(err, "contas")
	}
	out := make([]domain.Conta, len(rows))
	for i, row := range rows {
		out[i] = paraConta(db.BuscarContaRow(row))
	}
	return out, nil
}

func paraConta(r db.BuscarContaRow) domain.Conta {
	c := domain.Conta{ID: r.ID, Nome: r.Nome, PSP: r.Psp, ChavePix: r.ChavePix, Ambiente: domain.Ambiente(r.Ambiente), Ativa: r.Ativa == 1}
	if r.CertImpressaoDigital.Valid {
		c.Certificado = &domain.Certificado{
			Sujeito:          r.CertSujeito.String,
			ImpressaoDigital: r.CertImpressaoDigital.String,
			Validade:         time.Unix(r.CertValidadeS.Int64, 0).UTC(),
			// O aviso vale só para o certificado que foi avisado.
			VencimentoAvisado: r.CertAvisadoImpressao.Valid && r.CertAvisadoImpressao.String == r.CertImpressaoDigital.String,
		}
	}
	return c
}

func (r contas) Atualizar(ctx context.Context, c domain.Conta) error {
	n, err := r.q.AtualizarConta(ctx, db.AtualizarContaParams{
		Nome: c.Nome, ChavePix: c.ChavePix, Ambiente: string(c.Ambiente), Ativa: boolInt(c.Ativa), ID: c.ID,
	})
	if err != nil {
		return traduzir(err, "conta "+c.ID)
	}
	if n == 0 {
		return fmt.Errorf("conta %s: %w", c.ID, ports.ErrNaoEncontrado)
	}
	return nil
}

func (r contas) MarcarVencimentoAvisado(ctx context.Context, id, impressao string) error {
	n, err := r.q.MarcarVencimentoAvisado(ctx, db.MarcarVencimentoAvisadoParams{ID: id, CertImpressaoDigital: sql.NullString{String: impressao, Valid: true}})
	if err != nil {
		return traduzir(err, "conta "+id)
	}
	if n == 0 {
		return fmt.Errorf("conta %s com o certificado %s: %w", id, impressao, ports.ErrNaoEncontrado)
	}
	return nil
}

func (r contas) SalvarCredenciais(ctx context.Context, id string, cifrado []byte) error {
	n, err := r.q.SalvarCredenciais(ctx, db.SalvarCredenciaisParams{Credenciais: cifrado, ID: id})
	if err != nil {
		return traduzir(err, "credenciais da conta "+id)
	}
	if n == 0 {
		return fmt.Errorf("conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return nil
}

func (r contas) BuscarCredenciais(ctx context.Context, id string) ([]byte, error) {
	blob, err := r.q.BuscarCredenciais(ctx, id)
	if err != nil {
		return nil, traduzir(err, "conta "+id)
	}
	if blob == nil {
		return nil, fmt.Errorf("credenciais da conta %s: %w", id, ports.ErrNaoEncontrado)
	}
	return blob, nil
}

type cobrancas struct{ q *db.Queries }

func (r cobrancas) Criar(ctx context.Context, c domain.Cobranca) error {
	doc, nome := devedor(c.Devedor)
	err := r.q.CriarCobranca(ctx, db.CriarCobrancaParams{
		ContaID: c.ContaID, Txid: string(c.Txid), Chave: c.Chave, Status: string(c.Status), Valor: int64(c.Valor),
		PermiteAlteracao: boolInt(c.PermiteAlteracao), ExpiracaoS: int64(c.Expiracao / time.Second),
		DevedorDocumento: doc, DevedorNome: nome, SolicitacaoPagador: c.SolicitacaoPagador,
		Location: c.Location, BrCode: c.BRCode, CriadaEm: c.CriadaEm.UnixNano(),
	})
	return traduzir(err, "cobrança "+string(c.Txid))
}

func (r cobrancas) Buscar(ctx context.Context, contaID string, txid domain.Txid) (domain.Cobranca, error) {
	row, err := r.q.BuscarCobranca(ctx, db.BuscarCobrancaParams{ContaID: contaID, Txid: string(txid)})
	if err != nil {
		return domain.Cobranca{}, traduzir(err, "cobrança "+string(txid))
	}
	return paraCobranca(row), nil
}

func (r cobrancas) BuscarPorTxid(ctx context.Context, txid domain.Txid) (domain.Cobranca, error) {
	row, err := r.q.BuscarCobrancaPorTxid(ctx, string(txid))
	if err != nil {
		return domain.Cobranca{}, traduzir(err, "cobrança "+string(txid))
	}
	return paraCobranca(row), nil
}

func (r cobrancas) Contar(ctx context.Context, f ports.FiltroCobrancas) (int, error) {
	p := filtros(f)
	n, err := r.q.ContarCobrancas(ctx, db.ContarCobrancasParams{ContaID: p.ContaID, Status: p.Status, Documento: p.Documento, Inicio: p.Inicio, Fim: p.Fim})
	if err != nil {
		return 0, traduzir(err, "cobranças")
	}
	return int(n), nil
}

func (r cobrancas) Atualizar(ctx context.Context, c *domain.Cobranca) error {
	doc, nome := devedor(c.Devedor)
	n, err := r.q.AtualizarCobranca(ctx, db.AtualizarCobrancaParams{
		Status: string(c.Status), Valor: int64(c.Valor), PermiteAlteracao: boolInt(c.PermiteAlteracao),
		ExpiracaoS: int64(c.Expiracao / time.Second), DevedorDocumento: doc, DevedorNome: nome,
		SolicitacaoPagador: c.SolicitacaoPagador, Location: c.Location, BrCode: c.BRCode,
		ContaID: c.ContaID, Txid: string(c.Txid), Revisao: int64(c.Revisao),
	})
	if err != nil {
		return traduzir(err, "cobrança "+string(c.Txid))
	}
	if n == 0 {
		// Ou não existe, ou a revisão mudou: a busca desempata.
		if _, err := r.Buscar(ctx, c.ContaID, c.Txid); err != nil {
			return err
		}
		return fmt.Errorf("cobrança %s, revisão %d: %w", c.Txid, c.Revisao, ports.ErrConflito)
	}
	c.Revisao++
	return nil
}

func (r cobrancas) Listar(ctx context.Context, f ports.FiltroCobrancas) ([]domain.Cobranca, string, error) {
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
	p.DepoisDe, p.Limite, p.Pular = depoisDe, int64(limite)+1, int64(f.Pular)
	rows, err := r.q.ListarCobrancas(ctx, p)
	if err != nil {
		return nil, "", traduzir(err, "cobranças")
	}
	prox := ""
	if len(rows) > limite {
		rows = rows[:limite]
		prox = strconv.FormatInt(rows[limite-1].Seq, 10)
	}
	out := make([]domain.Cobranca, len(rows))
	for i, row := range rows {
		out[i] = paraCobranca(row)
	}
	return out, prox, nil
}

// filtros converte o filtro do port nos parâmetros opcionais do sqlc (nil = sem filtro).
func filtros(f ports.FiltroCobrancas) db.ListarCobrancasParams {
	var p db.ListarCobrancasParams
	if f.ContaID != "" {
		p.ContaID = f.ContaID
	}
	if f.Status != "" {
		p.Status = string(f.Status)
	}
	if f.Documento != "" {
		p.Documento = f.Documento
	}
	if !f.Janela.Inicio.IsZero() {
		p.Inicio = f.Janela.Inicio.UnixNano()
	}
	if !f.Janela.Fim.IsZero() {
		p.Fim = f.Janela.Fim.UnixNano()
	}
	return p
}

func devedor(d *domain.Devedor) (sql.NullString, sql.NullString) {
	if d == nil {
		return sql.NullString{}, sql.NullString{}
	}
	return sql.NullString{String: d.Documento, Valid: true}, sql.NullString{String: d.Nome, Valid: true}
}

func paraCobranca(r db.Cobranca) domain.Cobranca {
	c := domain.Cobranca{
		Txid: domain.Txid(r.Txid), ContaID: r.ContaID, Chave: r.Chave, Status: domain.StatusCobranca(r.Status), Valor: domain.Money(r.Valor),
		PermiteAlteracao: r.PermiteAlteracao == 1, Expiracao: time.Duration(r.ExpiracaoS) * time.Second,
		SolicitacaoPagador: r.SolicitacaoPagador, Location: r.Location, BRCode: r.BrCode,
		CriadaEm: time.Unix(0, r.CriadaEm).UTC(), Revisao: int(r.Revisao),
	}
	if r.DevedorDocumento.Valid {
		c.Devedor = &domain.Devedor{Documento: r.DevedorDocumento.String, Nome: r.DevedorNome.String}
	}
	return c
}

type outbox struct{ q *db.Queries }

func (r outbox) Adicionar(ctx context.Context, e domain.Evento) error {
	dados, err := json.Marshal(e.Dados)
	if err != nil {
		return fmt.Errorf("evento %s: serializando dados: %w", e.ID, err)
	}
	err = r.q.AdicionarEvento(ctx, db.AdicionarEventoParams{
		ID: e.ID, Tipo: string(e.Tipo), ContaID: e.ContaID, OcorridoEm: e.OcorridoEm.UnixNano(), Dados: string(dados),
	})
	return traduzir(err, "evento "+e.ID)
}
