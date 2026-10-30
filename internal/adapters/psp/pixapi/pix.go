package pixapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// itensPorPagina é quantos Pix pedir por página em GET /pix (o Efí aceita até 100).
const itensPorPagina = 100

// limitePaginas protege contra um laço sem fim se o banco repetir a paginação.
const limitePaginas = 1000

type pagadorJSON struct {
	CPF  string `json:"cpf,omitempty"`
	CNPJ string `json:"cnpj,omitempty"`
	Nome string `json:"nome,omitempty"`
}

type pixJSON struct {
	EndToEndID  string       `json:"endToEndId"`
	Txid        string       `json:"txid"`
	Valor       string       `json:"valor"`
	Chave       string       `json:"chave"`
	Horario     string       `json:"horario"`
	InfoPagador string       `json:"infoPagador"`
	Pagador     *pagadorJSON `json:"pagador"`
}

type listaPix struct {
	Parametros struct {
		Paginacao struct {
			PaginaAtual         int `json:"paginaAtual"`
			QuantidadeDePaginas int `json:"quantidadeDePaginas"`
		} `json:"paginacao"`
	} `json:"parametros"`
	Pix []pixJSON `json:"pix"`
}

func paraPix(conta domain.Conta, p pixJSON) (domain.PixRecebido, error) {
	if err := domain.ValidarEndToEndID(p.EndToEndID); err != nil {
		return domain.PixRecebido{}, err
	}
	valor, err := domain.ParseMoney(p.Valor)
	if err != nil || !valor.Valid() {
		return domain.PixRecebido{}, fmt.Errorf("%w: valor %q", domain.ErrPixInvalido, p.Valor)
	}
	horario, err := parseHorario(p.Horario)
	if err != nil {
		return domain.PixRecebido{}, fmt.Errorf("%w: horario %w", domain.ErrPixInvalido, err)
	}
	out := domain.PixRecebido{
		EndToEndID: p.EndToEndID, ContaID: conta.ID, Txid: domain.Txid(p.Txid), Valor: valor,
		Chave: p.Chave, Horario: horario, InfoPagador: p.InfoPagador,
	}
	if pg := p.Pagador; pg != nil && (pg.CPF != "" || pg.CNPJ != "" || pg.Nome != "") {
		doc := pg.CPF
		if doc == "" {
			doc = pg.CNPJ
		}
		out.Pagador = &domain.Devedor{Documento: doc, Nome: pg.Nome}
	}
	return out, nil
}

// ListarPix lê os Pix recebidos na janela (GET /pix), página por página. A
// spec filtra por [inicio, fim]; o port pede [Inicio, Fim), então quem cai
// exatamente em Fim fica para a próxima janela.
func (a *API) ListarPix(ctx context.Context, conta domain.Conta, j ports.Janela) ([]domain.PixRecebido, error) {
	var out []domain.PixRecebido
	for pagina := 0; pagina < limitePaginas; pagina++ {
		q := url.Values{}
		q.Set("inicio", j.Inicio.UTC().Format(time.RFC3339))
		q.Set("fim", j.Fim.UTC().Format(time.RFC3339))
		q.Set("paginacao.paginaAtual", strconv.Itoa(pagina))
		q.Set("paginacao.itensPorPagina", strconv.Itoa(itensPorPagina))
		var r listaPix
		if err := a.chamar(ctx, conta, http.MethodGet, a.cfg.Prefixo+"/pix?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for i, item := range r.Pix {
			pix, err := paraPix(conta, item)
			if err != nil {
				// Um Pix fora do formato invalida a consulta: melhor falhar que perder um pagamento.
				return nil, fmt.Errorf("%s: GET %s/pix página %d item %d: %w", a.cfg.Nome, a.cfg.Prefixo, pagina, i, err)
			}
			if !pix.Horario.Before(j.Inicio) && pix.Horario.Before(j.Fim) {
				out = append(out, pix)
			}
		}
		if pagina+1 >= r.Parametros.Paginacao.QuantidadeDePaginas {
			return out, nil
		}
	}
	return nil, fmt.Errorf("%s: GET %s/pix passou de %d páginas", a.cfg.Nome, a.cfg.Prefixo, limitePaginas)
}

type devolucaoJSON struct {
	ID       string `json:"id,omitempty"`
	RtrID    string `json:"rtrId,omitempty"`
	Valor    string `json:"valor"`
	Natureza string `json:"natureza,omitempty"`
	Status   string `json:"status,omitempty"`
	Motivo   string `json:"motivo,omitempty"`
	Horario  *struct {
		Solicitacao string `json:"solicitacao"`
		Liquidacao  string `json:"liquidacao,omitempty"`
	} `json:"horario,omitempty"`
	Descricao string `json:"descricao,omitempty"`
}

func (a *API) caminhoDevolucao(e2e, id string) string {
	return a.cfg.Prefixo + "/pix/" + url.PathEscape(e2e) + "/devolucao/" + url.PathEscape(id)
}

func paraDevolucao(conta domain.Conta, r devolucaoJSON, base domain.Devolucao) (domain.Devolucao, error) {
	d := base
	d.ContaID = conta.ID
	if r.ID != "" {
		d.ID = r.ID
	}
	if r.Valor != "" {
		v, err := domain.ParseMoney(r.Valor)
		if err != nil {
			return domain.Devolucao{}, fmt.Errorf("valor da devolução %q: %w", r.Valor, err)
		}
		d.Valor = v
	}
	d.Status = domain.StatusDevolucao(r.Status)
	if r.Motivo != "" {
		d.Motivo = r.Motivo
	}
	if r.Descricao != "" {
		d.Descricao = r.Descricao
	}
	if r.Natureza != "" {
		d.Natureza = domain.NaturezaDevolucao(r.Natureza)
	}
	d.RtrID = r.RtrID
	if r.Horario != nil && r.Horario.Solicitacao != "" {
		criada, err := parseHorario(r.Horario.Solicitacao)
		if err != nil {
			return domain.Devolucao{}, fmt.Errorf("horario.solicitacao: %w", err)
		}
		d.CriadaEm = criada
	}
	if r.Horario != nil && r.Horario.Liquidacao != "" {
		liquidada, err := parseHorario(r.Horario.Liquidacao)
		if err != nil {
			return domain.Devolucao{}, fmt.Errorf("horario.liquidacao: %w", err)
		}
		d.LiquidadaEm = liquidada
	}
	return d, nil
}

// ConsultarDevolucao lê a devolução (GET /pix/:e2eId/devolucao/:id).
func (a *API) ConsultarDevolucao(ctx context.Context, conta domain.Conta, e2e, id string) (domain.Devolucao, error) {
	var r devolucaoJSON
	if err := a.chamar(ctx, conta, http.MethodGet, a.caminhoDevolucao(e2e, id), nil, &r); err != nil {
		return domain.Devolucao{}, err
	}
	return paraDevolucao(conta, r, domain.Devolucao{ID: id, EndToEndID: e2e})
}

// Devolver pede a devolução (PUT /pix/:e2eId/devolucao/:id).
//
// Id repetido vem do Traduzir como ErrDevolucaoDuplicada. O port pede
// idempotência pelo id, então o adapter consulta a devolução existente e a
// devolve se o valor for o mesmo; valor diferente é ErrJaExiste.
func (a *API) Devolver(ctx context.Context, conta domain.Conta, d domain.Devolucao) (domain.Devolucao, error) {
	corpo := devolucaoJSON{Valor: d.Valor.String(), Natureza: string(d.Natureza), Descricao: d.Descricao}
	if a.cfg.ConsultarAntesDeDevolver {
		var existente devolucaoJSON
		switch err := a.chamar(ctx, conta, http.MethodGet, a.caminhoDevolucao(d.EndToEndID, d.ID), nil, &existente); {
		case err == nil && existente.Valor != corpo.Valor:
			return domain.Devolucao{}, fmt.Errorf("%s: devolução %s já existe com valor %s: %w", a.cfg.Nome, d.ID, existente.Valor, ports.ErrJaExiste)
		case err == nil:
			return paraDevolucao(conta, existente, d)
		case !errors.Is(err, ports.ErrNaoEncontrado):
			return domain.Devolucao{}, err
		}
	}
	var r devolucaoJSON
	err := a.chamar(ctx, conta, http.MethodPut, a.caminhoDevolucao(d.EndToEndID, d.ID), corpo, &r)
	if errors.Is(err, ErrDevolucaoDuplicada) {
		var existente devolucaoJSON
		if errGet := a.chamar(ctx, conta, http.MethodGet, a.caminhoDevolucao(d.EndToEndID, d.ID), nil, &existente); errGet != nil {
			return domain.Devolucao{}, err
		}
		if existente.Valor != corpo.Valor {
			return domain.Devolucao{}, fmt.Errorf("%s: devolução %s já existe com valor %s: %w", a.cfg.Nome, d.ID, existente.Valor, ports.ErrJaExiste)
		}
		return paraDevolucao(conta, existente, d)
	}
	if err != nil {
		return domain.Devolucao{}, err
	}
	return paraDevolucao(conta, r, d)
}
