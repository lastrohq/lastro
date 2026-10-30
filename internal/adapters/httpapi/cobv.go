package httpapi

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
)

// CriarCobv cria uma cobrança com vencimento com o txid do cliente.
func (s *Server) CriarCobv(ctx context.Context, r gen.CriarCobvRequestObject) (gen.CriarCobvResponseObject, error) {
	b := r.Body
	p := app.PedidoCobv{
		Txid: r.Txid, Chave: b.Chave, Valor: b.Valor.Original, Vencimento: b.Calendario.DataDeVencimento.String(),
		Devedor: pedidoDevedorCompleto(&b.Devedor), SolicitacaoPagador: deref(b.SolicitacaoPagador),
	}
	if b.Calendario.ValidadeAposVencimento != nil {
		p.ValidadeDias = ptr(int(*b.Calendario.ValidadeAposVencimento))
	}
	p.Multa, p.Juros, p.Abatimento, p.Desconto = valoresCobv(b.Valor)
	c, err := s.cobrancasV.Criar(ctx, p)
	if err != nil {
		return nil, err
	}
	return gen.CriarCobv201JSONResponse(s.cobvGerada(c)), nil
}

// ConsultarCobv devolve a cobv, sincronizando com o PSP se ela ainda estiver ativa.
func (s *Server) ConsultarCobv(ctx context.Context, r gen.ConsultarCobvRequestObject) (gen.ConsultarCobvResponseObject, error) {
	c, err := s.cobrancasV.Consultar(ctx, r.Txid)
	if err != nil {
		return nil, err
	}
	out := s.cobvCompleta(c)
	if out.Pix, err = s.pixDaCobranca(ctx, c.ContaID, c.Txid); err != nil {
		return nil, err
	}
	return gen.ConsultarCobv200JSONResponse(out), nil
}

// RevisarCobv aplica um PATCH na cobv.
func (s *Server) RevisarCobv(ctx context.Context, r gen.RevisarCobvRequestObject) (gen.RevisarCobvResponseObject, error) {
	b := r.Body
	rev := app.RevisaoCobv{Chave: b.Chave, SolicitacaoPagador: b.SolicitacaoPagador, Devedor: pedidoDevedorCompleto(b.Devedor)}
	if b.Calendario != nil {
		rev.Vencimento = ptr(b.Calendario.DataDeVencimento.String())
		if b.Calendario.ValidadeAposVencimento != nil {
			rev.ValidadeDias = ptr(int(*b.Calendario.ValidadeAposVencimento))
		}
	}
	if b.Valor != nil {
		rev.Valor = &b.Valor.Original
		rev.Multa, rev.Juros, rev.Abatimento, rev.Desconto = valoresCobv(*b.Valor)
	}
	rev.Remover = b.Status != nil && *b.Status == gen.CobVRevisadaStatusREMOVIDAPELOUSUARIORECEBEDOR
	c, err := s.cobrancasV.Revisar(ctx, r.Txid, rev)
	if err != nil {
		return nil, err
	}
	return gen.RevisarCobv200JSONResponse(s.cobvGerada(c)), nil
}

// ListarCobvs lista as cobvs criadas na janela.
func (s *Server) ListarCobvs(ctx context.Context, r gen.ListarCobvsRequestObject) (gen.ListarCobvsResponseObject, error) {
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
	pag, err := s.cobrancasV.Listar(ctx, q)
	if err != nil {
		return nil, err
	}
	cobs := make([]gen.CobVCompleta, len(pag.Cobs))
	for i, c := range pag.Cobs {
		cobs[i] = s.cobvCompleta(c)
		// Como na consulta: a lista da spec é de cobranças completas, com os Pix.
		if c.Status == domain.CobrancaConcluida {
			if cobs[i].Pix, err = s.pixDaCobranca(ctx, c.ContaID, c.Txid); err != nil {
				return nil, err
			}
		}
	}
	return gen.ListarCobvs200JSONResponse{
		Cobs: cobs,
		Parametros: gen.ParametrosConsulta{
			Inicio: q.Inicio, Fim: q.Fim, Cpf: r.Params.Cpf, Cnpj: r.Params.Cnpj,
			Status: (*string)(r.Params.Status), LocationPresente: r.Params.LocationPresente,
			Paginacao: gen.Paginacao{PaginaAtual: pag.Pagina, ItensPorPagina: pag.ItensPagina, QuantidadeDePaginas: pag.TotalPaginas, QuantidadeTotalDeItens: pag.TotalItens},
		},
	}, nil
}

func pedidoDevedorCompleto(d *gen.DevedorCompleto) *app.PedidoDevedorCompleto {
	if d == nil {
		return nil
	}
	p := &app.PedidoDevedorCompleto{
		PedidoDevedor: app.PedidoDevedor{Nome: d.Nome, CPF: deref(d.Cpf), CNPJ: deref(d.Cnpj)},
		Email:         deref(d.Email), Logradouro: deref(d.Logradouro), Cidade: deref(d.Cidade), UF: deref(d.Uf), CEP: deref(d.Cep),
	}
	return p
}

func valoresCobv(v gen.CobVValor) (multa, juros, abatimento *app.PedidoModalidade, desconto *app.PedidoDesconto) {
	if v.Multa != nil {
		multa = &app.PedidoModalidade{Modalidade: int(v.Multa.Modalidade), ValorPerc: v.Multa.ValorPerc}
	}
	if v.Juros != nil {
		juros = &app.PedidoModalidade{Modalidade: int(v.Juros.Modalidade), ValorPerc: v.Juros.ValorPerc}
	}
	if v.Abatimento != nil {
		abatimento = &app.PedidoModalidade{Modalidade: int(v.Abatimento.Modalidade), ValorPerc: v.Abatimento.ValorPerc}
	}
	if v.Desconto != nil {
		desconto = &app.PedidoDesconto{Modalidade: int(v.Desconto.Modalidade), ValorPerc: v.Desconto.ValorPerc}
		if v.Desconto.DescontoDataFixa != nil {
			for _, d := range *v.Desconto.DescontoDataFixa {
				desconto.DatasFixas = append(desconto.DatasFixas, app.PedidoDescontoData{Data: d.Data.String(), ValorPerc: d.ValorPerc})
			}
		}
	}
	return multa, juros, abatimento, desconto
}

func (s *Server) cobvGerada(c domain.CobrancaVencimento) gen.CobVGerada {
	g := gen.CobVGerada{
		Txid: c.Txid.String(), Revisao: int32(c.Revisao), Status: gen.CobVStatus(c.Status), Chave: c.Chave, //nolint:gosec // G115: revisão nunca passa de int32
		PixCopiaECola: ptr(c.BRCode),
		Devedor: gen.DevedorCompleto{
			Nome: c.Devedor.Nome, Email: vazioNil(c.Devedor.Email), Logradouro: vazioNil(c.Devedor.Logradouro),
			Cidade: vazioNil(c.Devedor.Cidade), Uf: vazioNil(c.Devedor.UF), Cep: vazioNil(c.Devedor.CEP),
		},
		Valor: gen.CobVValor{Original: c.Valor.String()},
	}
	if len(c.Devedor.Documento) == 11 {
		g.Devedor.Cpf = ptr(c.Devedor.Documento)
	} else {
		g.Devedor.Cnpj = ptr(c.Devedor.Documento)
	}
	g.Calendario.Criacao = c.CriadaEm
	g.Calendario.DataDeVencimento = openapi_types.Date{Time: c.Vencimento.Tempo()}
	g.Calendario.ValidadeAposVencimento = ptr(int32(c.ValidadeAposVencimento)) //nolint:gosec // G115: validade em dias cabe em int32
	if c.SolicitacaoPagador != "" {
		g.SolicitacaoPagador = ptr(c.SolicitacaoPagador)
	}
	if c.Location != "" {
		g.Loc = &gen.Loc{Location: ptr(c.Location), TipoCob: ptr(gen.Cobv)}
	}
	type mv = struct {
		Modalidade int32     `json:"modalidade"`
		ValorPerc  gen.Valor `json:"valorPerc"`
	}
	conv := func(m *domain.ValorModalidade) *mv {
		if m == nil {
			return nil
		}
		return &mv{Modalidade: int32(m.Modalidade), ValorPerc: m.ValorPerc.String()} //nolint:gosec // G115: modalidade é 1 a 8
	}
	g.Valor.Multa, g.Valor.Juros, g.Valor.Abatimento = conv(c.Multa), conv(c.Juros), conv(c.Abatimento)
	if d := c.Desconto; d != nil {
		g.Valor.Desconto = &struct {
			DescontoDataFixa *[]struct {
				Data      openapi_types.Date `json:"data"`
				ValorPerc gen.Valor          `json:"valorPerc"`
			} `json:"descontoDataFixa,omitempty"`
			Modalidade int32      `json:"modalidade"`
			ValorPerc  *gen.Valor `json:"valorPerc,omitempty"`
		}{Modalidade: int32(d.Modalidade)} //nolint:gosec // G115: modalidade é 1 a 6
		if len(d.DatasFixas) > 0 {
			datas := make([]struct {
				Data      openapi_types.Date `json:"data"`
				ValorPerc gen.Valor          `json:"valorPerc"`
			}, len(d.DatasFixas))
			for i, df := range d.DatasFixas {
				datas[i].Data, datas[i].ValorPerc = openapi_types.Date{Time: df.Data.Tempo()}, df.ValorPerc.String()
			}
			g.Valor.Desconto.DescontoDataFixa = &datas
		} else {
			g.Valor.Desconto.ValorPerc = ptr(d.ValorPerc.String())
		}
	}
	return g
}

// cobvCompleta é a cobvGerada; a lista de Pix é preenchida só na consulta individual.
func (s *Server) cobvCompleta(c domain.CobrancaVencimento) gen.CobVCompleta {
	g := s.cobvGerada(c)
	return gen.CobVCompleta{
		Calendario: g.Calendario, Chave: g.Chave, Devedor: g.Devedor, InfoAdicionais: g.InfoAdicionais, Loc: g.Loc,
		PixCopiaECola: g.PixCopiaECola, Recebedor: g.Recebedor, Revisao: g.Revisao, SolicitacaoPagador: g.SolicitacaoPagador,
		Status: g.Status, Txid: g.Txid, Valor: g.Valor,
	}
}

func vazioNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
