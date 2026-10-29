package efi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

type calendarioV struct {
	Criacao                string `json:"criacao,omitempty"`
	DataDeVencimento       string `json:"dataDeVencimento,omitempty"`
	ValidadeAposVencimento int    `json:"validadeAposVencimento,omitempty"`
}

type devedorV struct {
	CPF        string `json:"cpf,omitempty"`
	CNPJ       string `json:"cnpj,omitempty"`
	Nome       string `json:"nome,omitempty"`
	Email      string `json:"email,omitempty"`
	Logradouro string `json:"logradouro,omitempty"`
	Cidade     string `json:"cidade,omitempty"`
	UF         string `json:"uf,omitempty"`
	CEP        string `json:"cep,omitempty"`
}

type modalidade struct {
	Modalidade int    `json:"modalidade"`
	ValorPerc  string `json:"valorPerc,omitempty"`
}

type descontoData struct {
	Data      string `json:"data"`
	ValorPerc string `json:"valorPerc"`
}

type desconto struct {
	Modalidade       int            `json:"modalidade"`
	ValorPerc        string         `json:"valorPerc,omitempty"`
	DescontoDataFixa []descontoData `json:"descontoDataFixa,omitempty"`
}

type valorV struct {
	Original   string      `json:"original,omitempty"`
	Multa      *modalidade `json:"multa,omitempty"`
	Juros      *modalidade `json:"juros,omitempty"`
	Abatimento *modalidade `json:"abatimento,omitempty"`
	Desconto   *desconto   `json:"desconto,omitempty"`
}

// cobvEfi é o corpo de cobv no Efí, de ida e de volta.
type cobvEfi struct {
	Calendario         *calendarioV `json:"calendario,omitempty"`
	Txid               string       `json:"txid,omitempty"`
	Revisao            *int         `json:"revisao,omitempty"`
	Status             string       `json:"status,omitempty"`
	Devedor            *devedorV    `json:"devedor,omitempty"`
	Valor              *valorV      `json:"valor,omitempty"`
	Chave              string       `json:"chave,omitempty"`
	SolicitacaoPagador string       `json:"solicitacaoPagador,omitempty"`
	Location           string       `json:"location,omitempty"`
	// Loc é onde o Efí põe a location da cobv: diferente da cob, a resposta
	// da cobv não traz o campo location no topo (visto na homologação).
	Loc           *locEfi `json:"loc,omitempty"`
	PixCopiaECola string  `json:"pixCopiaECola,omitempty"`
}

// locEfi é o payload location que o Efí devolve dentro da cobrança.
type locEfi struct {
	Location string `json:"location"`
}

func paraModalidade(m *domain.ValorModalidade) *modalidade {
	if m == nil {
		return nil
	}
	return &modalidade{Modalidade: m.Modalidade, ValorPerc: m.ValorPerc.String()}
}

func deModalidade(m *modalidade) (*domain.ValorModalidade, error) {
	if m == nil {
		return nil, nil
	}
	v, err := domain.ParseMoney(m.ValorPerc)
	if err != nil {
		return nil, fmt.Errorf("valorPerc %q: %w", m.ValorPerc, err)
	}
	return &domain.ValorModalidade{Modalidade: m.Modalidade, ValorPerc: v}, nil
}

func corpoCobv(conta domain.Conta, c domain.CobrancaVencimento) cobvEfi {
	if c.Chave == "" {
		c.Chave = conta.ChavePix
	}
	d := c.Devedor
	dv := &devedorV{Nome: d.Nome, Email: d.Email, Logradouro: d.Logradouro, Cidade: d.Cidade, UF: d.UF, CEP: d.CEP}
	if len(d.Documento) == 11 {
		dv.CPF = d.Documento
	} else {
		dv.CNPJ = d.Documento
	}
	v := &valorV{
		Original:   c.Valor.String(),
		Multa:      paraModalidade(c.Multa),
		Juros:      paraModalidade(c.Juros),
		Abatimento: paraModalidade(c.Abatimento),
	}
	if c.Desconto != nil {
		v.Desconto = &desconto{Modalidade: c.Desconto.Modalidade}
		if len(c.Desconto.DatasFixas) > 0 {
			for _, df := range c.Desconto.DatasFixas {
				v.Desconto.DescontoDataFixa = append(v.Desconto.DescontoDataFixa, descontoData{Data: df.Data.String(), ValorPerc: df.ValorPerc.String()})
			}
		} else {
			v.Desconto.ValorPerc = c.Desconto.ValorPerc.String()
		}
	}
	return cobvEfi{
		Calendario:         &calendarioV{DataDeVencimento: c.Vencimento.String(), ValidadeAposVencimento: c.ValidadeAposVencimento},
		Devedor:            dv,
		Valor:              v,
		Chave:              c.Chave,
		SolicitacaoPagador: c.SolicitacaoPagador,
	}
}

// paraCobv traduz a resposta do Efí; base completa o que o Efí não devolve.
func paraCobv(conta domain.Conta, r cobvEfi, base domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	c := base
	c.ContaID = conta.ID
	if r.Txid != "" {
		c.Txid = domain.Txid(r.Txid)
	}
	if r.Status != "" {
		c.Status = domain.StatusCobranca(r.Status)
	}
	switch {
	case r.Location != "":
		c.Location = r.Location
	case r.Loc != nil && r.Loc.Location != "":
		c.Location = r.Loc.Location
	}
	if r.PixCopiaECola != "" {
		c.BRCode = r.PixCopiaECola
	}
	if r.Revisao != nil {
		c.Revisao = *r.Revisao
	}
	if r.Chave != "" {
		c.Chave = r.Chave
	}
	c.SolicitacaoPagador = r.SolicitacaoPagador
	if cal := r.Calendario; cal != nil {
		if cal.DataDeVencimento != "" {
			d, err := domain.ParseData(cal.DataDeVencimento)
			if err != nil {
				return domain.CobrancaVencimento{}, fmt.Errorf("efi: calendario.dataDeVencimento: %w", err)
			}
			c.Vencimento = d
		}
		c.ValidadeAposVencimento = cal.ValidadeAposVencimento
		if cal.Criacao != "" {
			criada, err := parseHorario(cal.Criacao)
			if err != nil {
				return domain.CobrancaVencimento{}, fmt.Errorf("efi: calendario.criacao: %w", err)
			}
			c.CriadaEm = criada
		}
	}
	if d := r.Devedor; d != nil {
		doc := d.CPF
		if doc == "" {
			doc = d.CNPJ
		}
		c.Devedor = domain.DevedorCompleto{Devedor: domain.Devedor{Documento: doc, Nome: d.Nome}, Email: d.Email, Logradouro: d.Logradouro, Cidade: d.Cidade, UF: d.UF, CEP: d.CEP}
	}
	if v := r.Valor; v != nil {
		var err error
		if v.Original != "" {
			if c.Valor, err = domain.ParseMoney(v.Original); err != nil {
				return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.original %q: %w", v.Original, err)
			}
		}
		if c.Multa, err = deModalidade(v.Multa); err != nil {
			return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.multa: %w", err)
		}
		if c.Juros, err = deModalidade(v.Juros); err != nil {
			return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.juros: %w", err)
		}
		if c.Abatimento, err = deModalidade(v.Abatimento); err != nil {
			return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.abatimento: %w", err)
		}
		c.Desconto = nil
		if v.Desconto != nil {
			des := &domain.Desconto{Modalidade: v.Desconto.Modalidade}
			if v.Desconto.ValorPerc != "" {
				if des.ValorPerc, err = domain.ParseMoney(v.Desconto.ValorPerc); err != nil {
					return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.desconto: %w", err)
				}
			}
			for _, df := range v.Desconto.DescontoDataFixa {
				data, err := domain.ParseData(df.Data)
				if err != nil {
					return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.desconto.descontoDataFixa: %w", err)
				}
				vp, err := domain.ParseMoney(df.ValorPerc)
				if err != nil {
					return domain.CobrancaVencimento{}, fmt.Errorf("efi: valor.desconto.descontoDataFixa: %w", err)
				}
				des.DatasFixas = append(des.DatasFixas, domain.DescontoData{Data: data, ValorPerc: vp})
			}
			c.Desconto = des
		}
	}
	return c, nil
}

func caminhoCobv(txid domain.Txid) string { return "/v2/cobv/" + url.PathEscape(string(txid)) }

// CriarCobv cria a cobrança com vencimento (PUT /v2/cobv/:txid).
//
// Desvio do Efí, visto na homologação em 29/10/2026: PUT num txid que já
// existe não dá erro; sobrescreve a cobv e soma uma revisão (na cob, o Efí
// recusa). Para cumprir o port (txid repetido é ErrJaExiste) sem apagar a
// cobrança de ninguém, o adapter consulta o txid antes e só cria se ele não
// existir. Entre a consulta e o PUT sobra uma janela curta; o Lastro já
// impede dois pedidos com o mesmo txid antes de chegar aqui.
func (p *PSP) CriarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	switch _, err := p.ConsultarCobv(ctx, conta, c.Txid); {
	case err == nil:
		return domain.CobrancaVencimento{}, fmt.Errorf("efi: cobv %s: %w", c.Txid, ports.ErrJaExiste)
	case !errors.Is(err, ports.ErrNaoEncontrado):
		return domain.CobrancaVencimento{}, err
	}
	var r cobvEfi
	if err := p.chamar(ctx, conta, http.MethodPut, caminhoCobv(c.Txid), corpoCobv(conta, c), &r); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	return paraCobv(conta, r, c)
}

// ConsultarCobv lê a cobrança com vencimento (GET /v2/cobv/:txid).
func (p *PSP) ConsultarCobv(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.CobrancaVencimento, error) {
	var r cobvEfi
	if err := p.chamar(ctx, conta, http.MethodGet, caminhoCobv(txid), nil, &r); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	return paraCobv(conta, r, domain.CobrancaVencimento{Txid: txid})
}

// RevisarCobv envia o novo estado (PATCH /v2/cobv/:txid). Remoção pelo
// recebedor vai sozinha, só com o status.
func (p *PSP) RevisarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	corpo := corpoCobv(conta, c)
	corpo.Chave = ""
	if c.Status == domain.CobrancaRemovidaPeloUsuarioRecebedor {
		corpo = cobvEfi{Status: string(c.Status)}
	}
	var r cobvEfi
	if err := p.chamar(ctx, conta, http.MethodPatch, caminhoCobv(c.Txid), corpo, &r); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	return paraCobv(conta, r, c)
}
