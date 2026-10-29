package efi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// segundos aceita número ou string: o Efí envia expiracao como string no GET.
type segundos int64

func (s *segundos) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	if len(b) == 0 || string(b) == "null" {
		*s = 0
		return nil
	}
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("expiracao %q não é um número de segundos", b)
	}
	*s = segundos(n)
	return nil
}

type calendario struct {
	Criacao   string   `json:"criacao,omitempty"`
	Expiracao segundos `json:"expiracao,omitempty"`
}

type devedor struct {
	CPF  string `json:"cpf,omitempty"`
	CNPJ string `json:"cnpj,omitempty"`
	Nome string `json:"nome,omitempty"`
}

type valor struct {
	Original string `json:"original"`
	// ModalidadeAlteracao 1 deixa o pagador mudar o valor (Pix Saque/Troco e afins).
	ModalidadeAlteracao int `json:"modalidadeAlteracao,omitempty"`
}

// cobEfi é o corpo de cob no Efí, de ida e de volta.
type cobEfi struct {
	Calendario         *calendario `json:"calendario,omitempty"`
	Txid               string      `json:"txid,omitempty"`
	Revisao            *int        `json:"revisao,omitempty"`
	Status             string      `json:"status,omitempty"`
	Devedor            *devedor    `json:"devedor,omitempty"`
	Valor              *valor      `json:"valor,omitempty"`
	Chave              string      `json:"chave,omitempty"`
	SolicitacaoPagador string      `json:"solicitacaoPagador,omitempty"`
	Location           string      `json:"location,omitempty"`
	PixCopiaECola      string      `json:"pixCopiaECola,omitempty"`
}

func paraDevedor(d *domain.Devedor) *devedor {
	if d == nil {
		return nil
	}
	out := &devedor{Nome: d.Nome}
	if len(d.Documento) == 11 {
		out.CPF = d.Documento
	} else {
		out.CNPJ = d.Documento
	}
	return out
}

func paraValor(v domain.Money, permiteAlteracao bool) *valor {
	out := &valor{Original: v.String()}
	if permiteAlteracao {
		out.ModalidadeAlteracao = 1
	}
	return out
}

// corpoCob monta o PUT /v2/cob/:txid a partir da cobrança do domínio.
func corpoCob(c domain.Cobranca) cobEfi {
	return cobEfi{
		Calendario:         &calendario{Expiracao: segundos(c.Expiracao / time.Second)},
		Devedor:            paraDevedor(c.Devedor),
		Valor:              paraValor(c.Valor, c.PermiteAlteracao),
		Chave:              c.Chave,
		SolicitacaoPagador: c.SolicitacaoPagador,
	}
}

// paraCobranca traduz a resposta do Efí; base completa o que o Efí não devolve.
func paraCobranca(conta domain.Conta, r cobEfi, base domain.Cobranca) (domain.Cobranca, error) {
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
	if r.Valor != nil {
		v, err := domain.ParseMoney(r.Valor.Original)
		if err != nil {
			return domain.Cobranca{}, fmt.Errorf("efi: valor.original %q: %w", r.Valor.Original, err)
		}
		c.Valor, c.PermiteAlteracao = v, r.Valor.ModalidadeAlteracao == 1
	}
	if r.Devedor != nil {
		doc := r.Devedor.CPF
		if doc == "" {
			doc = r.Devedor.CNPJ
		}
		c.Devedor = &domain.Devedor{Documento: doc, Nome: r.Devedor.Nome}
	} else {
		c.Devedor = nil
	}
	if r.Calendario != nil {
		if r.Calendario.Expiracao > 0 {
			c.Expiracao = time.Duration(r.Calendario.Expiracao) * time.Second
		}
		if r.Calendario.Criacao != "" {
			criada, err := parseHorario(r.Calendario.Criacao)
			if err != nil {
				return domain.Cobranca{}, fmt.Errorf("efi: calendario.criacao: %w", err)
			}
			c.CriadaEm = criada
		}
	}
	return c, nil
}

// parseHorario lê os instantes do Efí (RFC 3339, com ou sem fração de segundo) em UTC.
func parseHorario(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q não é RFC 3339", s)
	}
	return t.UTC(), nil
}

func caminhoCob(txid domain.Txid) string { return "/v2/cob/" + url.PathEscape(string(txid)) }

// CriarCob cria a cobrança com o txid do Lastro (PUT /v2/cob/:txid).
func (p *PSP) CriarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	var r cobEfi
	if err := p.chamar(ctx, conta, http.MethodPut, caminhoCob(c.Txid), corpoCob(c), &r); err != nil {
		return domain.Cobranca{}, err
	}
	return paraCobranca(conta, r, c)
}

// ConsultarCob lê a cobrança (GET /v2/cob/:txid).
func (p *PSP) ConsultarCob(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.Cobranca, error) {
	var r cobEfi
	if err := p.chamar(ctx, conta, http.MethodGet, caminhoCob(txid), nil, &r); err != nil {
		return domain.Cobranca{}, err
	}
	return paraCobranca(conta, r, domain.Cobranca{Txid: txid})
}

// RevisarCob envia o novo estado (PATCH /v2/cob/:txid). Remoção pelo recebedor
// vai sozinha, só com o status, como no exemplo do Efí.
func (p *PSP) RevisarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	corpo := corpoCob(c)
	corpo.Chave = "" // a chave não muda numa revisão
	if c.Status == domain.CobrancaRemovidaPeloUsuarioRecebedor {
		corpo = cobEfi{Status: string(c.Status)}
	}
	var r cobEfi
	if err := p.chamar(ctx, conta, http.MethodPatch, caminhoCob(c.Txid), corpo, &r); err != nil {
		return domain.Cobranca{}, err
	}
	return paraCobranca(conta, r, c)
}

func naoImplementado(capacidade ports.Capacidade) error {
	return fmt.Errorf("efi: %w: %s ainda não implementado neste adapter", ports.ErrNaoSuportado, capacidade)
}

// ConfigurarWebhook ainda não está implementado no adapter do Efí.
func (p *PSP) ConfigurarWebhook(context.Context, domain.Conta, string) error {
	return naoImplementado(ports.CapWebhook)
}

// InterpretarWebhook ainda não está implementado no adapter do Efí.
func (p *PSP) InterpretarWebhook(domain.Conta, []byte) ([]domain.PixRecebido, error) {
	return nil, naoImplementado(ports.CapWebhook)
}

var _ json.Unmarshaler = (*segundos)(nil)
