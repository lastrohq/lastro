package httpapi

import (
	"context"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
)

// CriarCob cria uma cob com txid gerado pelo Lastro.
func (s *Server) CriarCob(ctx context.Context, r gen.CriarCobRequestObject) (gen.CriarCobResponseObject, error) {
	c, err := s.cobrancas.Criar(ctx, pedidoCob("", r.Body))
	if err != nil {
		return nil, err
	}
	return gen.CriarCob201JSONResponse(s.cobGerada(c)), nil
}

// CriarCobComTxid cria uma cob com o txid escolhido pelo cliente.
func (s *Server) CriarCobComTxid(ctx context.Context, r gen.CriarCobComTxidRequestObject) (gen.CriarCobComTxidResponseObject, error) {
	c, err := s.cobrancas.Criar(ctx, pedidoCob(r.Txid, r.Body))
	if err != nil {
		return nil, err
	}
	return gen.CriarCobComTxid201JSONResponse(s.cobGerada(c)), nil
}

// ConsultarCob devolve a cob, sincronizando com o PSP se ela ainda estiver ativa.
func (s *Server) ConsultarCob(ctx context.Context, r gen.ConsultarCobRequestObject) (gen.ConsultarCobResponseObject, error) {
	c, err := s.cobrancas.Consultar(ctx, r.Txid)
	if err != nil {
		return nil, err
	}
	out := s.cobCompleta(c)
	if out.Pix, err = s.pixDaCobranca(ctx, c.ContaID, c.Txid); err != nil {
		return nil, err
	}
	return gen.ConsultarCob200JSONResponse(out), nil
}

// RevisarCob aplica um PATCH na cob.
func (s *Server) RevisarCob(ctx context.Context, r gen.RevisarCobRequestObject) (gen.RevisarCobResponseObject, error) {
	b := r.Body
	rev := app.RevisaoCob{SolicitacaoPagador: b.SolicitacaoPagador, Devedor: pedidoDevedor(b.Devedor), Chave: b.Chave}
	if b.Calendario != nil && b.Calendario.Expiracao != nil {
		rev.ExpiracaoSegundos = ptr(int(*b.Calendario.Expiracao))
	}
	if b.Valor != nil {
		rev.Valor = &b.Valor.Original
		if b.Valor.ModalidadeAlteracao != nil {
			rev.PermiteAlteracao = ptr(*b.Valor.ModalidadeAlteracao == 1)
		}
	}
	rev.Remover = b.Status != nil && *b.Status == gen.CobRevisadaStatusREMOVIDAPELOUSUARIORECEBEDOR
	c, err := s.cobrancas.Revisar(ctx, r.Txid, rev)
	if err != nil {
		return nil, err
	}
	return gen.RevisarCob200JSONResponse(s.cobGerada(c)), nil
}

// ListarCobs lista as cobs criadas na janela, paginadas como a spec.
func (s *Server) ListarCobs(ctx context.Context, r gen.ListarCobsRequestObject) (gen.ListarCobsResponseObject, error) {
	q := app.ConsultaCobs{Inicio: r.Params.Inicio, Fim: r.Params.Fim}
	if r.Params.Cpf != nil {
		q.Documento = *r.Params.Cpf
	}
	if r.Params.Cnpj != nil {
		q.Documento = *r.Params.Cnpj
	}
	if r.Params.Status != nil {
		q.Status = domain.StatusCobranca(*r.Params.Status)
	}
	if r.Params.PaginacaoPaginaAtual != nil {
		q.Pagina = int(*r.Params.PaginacaoPaginaAtual)
	}
	if r.Params.PaginacaoItensPorPagina != nil {
		q.ItensPagina = int(*r.Params.PaginacaoItensPorPagina)
	}
	pag, err := s.cobrancas.Listar(ctx, q)
	if err != nil {
		return nil, err
	}
	cobs := make([]gen.CobCompleta, len(pag.Cobs))
	for i, c := range pag.Cobs {
		cobs[i] = s.cobCompleta(c)
	}
	return gen.ListarCobs200JSONResponse{
		Cobs: cobs,
		Parametros: gen.ParametrosConsulta{
			Inicio: q.Inicio, Fim: q.Fim, Cpf: r.Params.Cpf, Cnpj: r.Params.Cnpj,
			Status: (*string)(r.Params.Status), LocationPresente: r.Params.LocationPresente,
			Paginacao: gen.Paginacao{PaginaAtual: pag.Pagina, ItensPorPagina: pag.ItensPagina, QuantidadeDePaginas: pag.TotalPaginas, QuantidadeTotalDeItens: pag.TotalItens},
		},
	}, nil
}

func pedidoCob(txid string, b *gen.CobSolicitada) app.PedidoCob {
	p := app.PedidoCob{Txid: txid, Chave: b.Chave, Valor: b.Valor.Original, Devedor: pedidoDevedor(b.Devedor)}
	if b.Valor.ModalidadeAlteracao != nil {
		p.PermiteAlteracao = *b.Valor.ModalidadeAlteracao == 1
	}
	if b.Calendario.Expiracao != nil {
		p.ExpiracaoSegundos = ptr(int(*b.Calendario.Expiracao))
	}
	if b.SolicitacaoPagador != nil {
		p.SolicitacaoPagador = *b.SolicitacaoPagador
	}
	return p
}

func pedidoDevedor(d *gen.Devedor) *app.PedidoDevedor {
	if d == nil {
		return nil
	}
	p := &app.PedidoDevedor{Nome: d.Nome}
	if d.Cpf != nil {
		p.CPF = *d.Cpf
	}
	if d.Cnpj != nil {
		p.CNPJ = *d.Cnpj
	}
	return p
}

func (s *Server) cobGerada(c domain.Cobranca) gen.CobGerada {
	g := gen.CobGerada{
		Txid: c.Txid.String(), Revisao: int32(c.Revisao), Status: gen.CobStatus(c.Status), Chave: c.Chave, //nolint:gosec // G115: revisão nunca passa de int32
		Valor:    gen.CobValor{Original: c.Valor.String()},
		Location: ptr(c.Location), PixCopiaECola: ptr(c.BRCode),
		Expirada: ptr(c.Expirada(s.clock.Now())),
	}
	g.Calendario.Criacao = c.CriadaEm
	g.Calendario.Expiracao = int32(c.Expiracao.Seconds())
	if c.PermiteAlteracao {
		g.Valor.ModalidadeAlteracao = ptr(int32(1))
	}
	if c.SolicitacaoPagador != "" {
		g.SolicitacaoPagador = ptr(c.SolicitacaoPagador)
	}
	if c.Devedor != nil {
		g.Devedor = &gen.Devedor{Nome: c.Devedor.Nome}
		if len(c.Devedor.Documento) == 11 {
			g.Devedor.Cpf = ptr(c.Devedor.Documento)
		} else {
			g.Devedor.Cnpj = ptr(c.Devedor.Documento)
		}
	}
	return g
}

// pixDaCobranca monta a lista "pix" da CobCompleta e da CobVCompleta.
func (s *Server) pixDaCobranca(ctx context.Context, contaID string, txid domain.Txid) (*[]gen.Pix, error) {
	pixes, err := s.webhooks.Recebimentos.DaCobranca(ctx, contaID, txid)
	if err != nil || len(pixes) == 0 {
		return nil, err
	}
	out := make([]gen.Pix, len(pixes))
	for i, p := range pixes {
		out[i] = gen.Pix{EndToEndId: p.EndToEndID, Txid: ptr(string(p.Txid)), Valor: p.Valor.String(), Horario: p.Horario, Chave: vazioNil(p.Chave), InfoPagador: vazioNil(p.InfoPagador)}
	}
	return &out, nil
}

// cobCompleta é a cobGerada; a lista de Pix é preenchida só na consulta individual.
func (s *Server) cobCompleta(c domain.Cobranca) gen.CobCompleta {
	g := s.cobGerada(c)
	return gen.CobCompleta{
		Calendario: g.Calendario, Chave: g.Chave, Devedor: g.Devedor, Expirada: g.Expirada,
		InfoAdicionais: g.InfoAdicionais, Loc: g.Loc, Location: g.Location, PixCopiaECola: g.PixCopiaECola,
		Revisao: g.Revisao, SolicitacaoPagador: g.SolicitacaoPagador, Status: g.Status, Txid: g.Txid, Valor: g.Valor,
	}
}
