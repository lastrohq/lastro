package pixapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// segundos aceita número ou string: há banco (o Efí) que envia expiracao como string no GET.
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

// cobJSON é o corpo de cob da spec, de ida e de volta.
type cobJSON struct {
	Calendario         *calendario `json:"calendario,omitempty"`
	Txid               string      `json:"txid,omitempty"`
	Revisao            *int        `json:"revisao,omitempty"`
	Status             string      `json:"status,omitempty"`
	Devedor            *devedor    `json:"devedor,omitempty"`
	Valor              *valor      `json:"valor,omitempty"`
	Chave              string      `json:"chave,omitempty"`
	SolicitacaoPagador string      `json:"solicitacaoPagador,omitempty"`
	Location           string      `json:"location,omitempty"`
	Loc                *locJSON    `json:"loc,omitempty"`
	PixCopiaECola      string      `json:"pixCopiaECola,omitempty"`
	// Brcode é onde o Sicoob devolve o BR Code, em vez do pixCopiaECola.
	Brcode string `json:"brcode,omitempty"`
	// TextoImagemQRcode é onde o BB devolve o BR Code.
	TextoImagemQRcode string `json:"textoImagemQRcode,omitempty"`
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

// corpoCob monta o PUT /cob/:txid a partir da cobrança do domínio. Sem chave
// na cobrança, vale a chave da conta (a spec exige a chave).
func corpoCob(conta domain.Conta, c domain.Cobranca) cobJSON {
	if c.Chave == "" {
		c.Chave = conta.ChavePix
	}
	return cobJSON{
		Calendario:         &calendario{Expiracao: segundos(c.Expiracao / time.Second)},
		Devedor:            paraDevedor(c.Devedor),
		Valor:              paraValor(c.Valor, c.PermiteAlteracao),
		Chave:              c.Chave,
		SolicitacaoPagador: c.SolicitacaoPagador,
	}
}

// paraCobranca traduz a resposta do banco; base completa o que ele não devolve.
func paraCobranca(conta domain.Conta, r cobJSON, base domain.Cobranca) (domain.Cobranca, error) {
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
	switch {
	case r.PixCopiaECola != "":
		c.BRCode = r.PixCopiaECola
	case r.Brcode != "":
		c.BRCode = r.Brcode
	case r.TextoImagemQRcode != "":
		c.BRCode = r.TextoImagemQRcode
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
			return domain.Cobranca{}, fmt.Errorf("valor.original %q: %w", r.Valor.Original, err)
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
				return domain.Cobranca{}, fmt.Errorf("calendario.criacao: %w", err)
			}
			c.CriadaEm = criada
		}
	}
	return c, nil
}

// parseHorario lê os instantes do banco (RFC 3339, com ou sem fração de segundo) em UTC.
func parseHorario(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%q não é RFC 3339", s)
	}
	return t.UTC(), nil
}

func (a *API) caminhoCob(txid domain.Txid) string {
	return a.cfg.Prefixo + "/cob/" + url.PathEscape(string(txid))
}

// CriarCob cria a cobrança com o txid do Vintém (PUT /cob/:txid). Com
// ConsultarAntesDeCriar.Cob, consulta o txid antes: txid repetido é
// ErrJaExiste mesmo no banco que sobrescreveria ou não sinalizaria.
func (a *API) CriarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	if a.cfg.ConsultarAntesDeCriar.Cob {
		if err := a.naoExiste(a.ConsultarCob(ctx, conta, c.Txid)); err != nil {
			return domain.Cobranca{}, fmt.Errorf("%s: cob %s: %w", a.cfg.Nome, c.Txid, err)
		}
	}
	var r cobJSON
	if err := a.chamar(ctx, conta, http.MethodPut, a.caminhoCob(c.Txid), corpoCob(conta, c), &r); err != nil {
		return domain.Cobranca{}, err
	}
	return paraCobranca(conta, r, c)
}

// ConsultarCob lê a cobrança (GET /cob/:txid).
func (a *API) ConsultarCob(ctx context.Context, conta domain.Conta, txid domain.Txid) (domain.Cobranca, error) {
	var r cobJSON
	if err := a.chamar(ctx, conta, http.MethodGet, a.caminhoCob(txid), nil, &r); err != nil {
		return domain.Cobranca{}, err
	}
	return paraCobranca(conta, r, domain.Cobranca{Txid: txid})
}

// RevisarCob envia o novo estado (PATCH /cob/:txid). Remoção pelo recebedor
// vai sozinha, só com o status, como no exemplo da spec.
func (a *API) RevisarCob(ctx context.Context, conta domain.Conta, c domain.Cobranca) (domain.Cobranca, error) {
	corpo := corpoCob(conta, c)
	corpo.Chave = "" // a chave não muda numa revisão
	if c.Status == domain.CobrancaRemovidaPeloUsuarioRecebedor {
		corpo = cobJSON{Status: string(c.Status)}
	}
	var r cobJSON
	if err := a.chamar(ctx, conta, http.MethodPatch, a.caminhoCob(c.Txid), corpo, &r); err != nil {
		return domain.Cobranca{}, err
	}
	return paraCobranca(conta, r, c)
}

var _ json.Unmarshaler = (*segundos)(nil)

// naoExiste recebe o resultado de uma consulta feita antes de criar: achou é
// ErrJaExiste, não achou é nil, outro erro passa adiante.
func (a *API) naoExiste(_ any, err error) error {
	switch {
	case err == nil:
		return ports.ErrJaExiste
	case errors.Is(err, ports.ErrNaoEncontrado):
		return nil
	}
	return err
}

func escapar(s string) string { return url.PathEscape(s) }
