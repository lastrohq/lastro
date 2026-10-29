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
	PixCopiaECola      string       `json:"pixCopiaECola,omitempty"`
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

func corpoCobv(c domain.CobrancaVencimento) cobvEfi {
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
	if r.Location != "" {
		c.Location = r.Location
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
// O Efí não documenta um nome de erro para txid repetido na cobv (só um 400
// "Cobrança inválida."). Quando o PUT falha com 4xx, o adapter consulta o txid:
// se a cobv já existe, o erro é ErrJaExiste. A confirmar na homologação.
func (p *PSP) CriarCobv(ctx context.Context, conta domain.Conta, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	var r cobvEfi
	err := p.chamar(ctx, conta, http.MethodPut, caminhoCobv(c.Txid), corpoCobv(c), &r)
	if err != nil {
		var e *erroHTTP
		if errors.As(err, &e) && e.status >= 400 && e.status < 500 && !errors.Is(err, ports.ErrJaExiste) {
			if _, errGet := p.ConsultarCobv(ctx, conta, c.Txid); errGet == nil {
				return domain.CobrancaVencimento{}, fmt.Errorf("efi: cobv %s: %w (%w)", c.Txid, ports.ErrJaExiste, err)
			}
		}
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
	corpo := corpoCobv(c)
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
