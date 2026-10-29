package efi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// itensPorPagina é o máximo que o Efí devolve por página em GET /v2/pix.
const itensPorPagina = 100

// limitePaginas protege contra um laço sem fim se o Efí repetir a paginação.
const limitePaginas = 1000

type pagadorEfi struct {
	CPF  string `json:"cpf,omitempty"`
	CNPJ string `json:"cnpj,omitempty"`
	Nome string `json:"nome,omitempty"`
}

type pixEfi struct {
	EndToEndID  string      `json:"endToEndId"`
	Txid        string      `json:"txid"`
	Valor       string      `json:"valor"`
	Chave       string      `json:"chave"`
	Horario     string      `json:"horario"`
	InfoPagador string      `json:"infoPagador"`
	Pagador     *pagadorEfi `json:"pagador"`
}

type listaPix struct {
	Parametros struct {
		Paginacao struct {
			PaginaAtual         int `json:"paginaAtual"`
			QuantidadeDePaginas int `json:"quantidadeDePaginas"`
		} `json:"paginacao"`
	} `json:"parametros"`
	Pix []pixEfi `json:"pix"`
}

func paraPix(conta domain.Conta, p pixEfi) (domain.PixRecebido, error) {
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

// ListarPix lê os Pix recebidos na janela (GET /v2/pix), página por página. O
// Efí filtra por [inicio, fim]; o port pede [Inicio, Fim), então quem cai
// exatamente em Fim fica para a próxima janela.
func (p *PSP) ListarPix(ctx context.Context, conta domain.Conta, j ports.Janela) ([]domain.PixRecebido, error) {
	var out []domain.PixRecebido
	for pagina := 0; pagina < limitePaginas; pagina++ {
		q := url.Values{}
		q.Set("inicio", j.Inicio.UTC().Format(time.RFC3339))
		q.Set("fim", j.Fim.UTC().Format(time.RFC3339))
		q.Set("paginacao.paginaAtual", strconv.Itoa(pagina))
		q.Set("paginacao.itensPorPagina", strconv.Itoa(itensPorPagina))
		var r listaPix
		if err := p.chamar(ctx, conta, http.MethodGet, "/v2/pix?"+q.Encode(), nil, &r); err != nil {
			return nil, err
		}
		for i, item := range r.Pix {
			pix, err := paraPix(conta, item)
			if err != nil {
				// Um Pix fora do formato invalida a consulta: melhor falhar que perder um pagamento.
				return nil, fmt.Errorf("efi: GET /v2/pix página %d item %d: %w", pagina, i, err)
			}
			if !pix.Horario.Before(j.Inicio) && pix.Horario.Before(j.Fim) {
				out = append(out, pix)
			}
		}
		if pagina+1 >= r.Parametros.Paginacao.QuantidadeDePaginas {
			return out, nil
		}
	}
	return nil, fmt.Errorf("efi: GET /v2/pix passou de %d páginas", limitePaginas)
}

type devolucaoEfi struct {
	ID      string `json:"id,omitempty"`
	RtrID   string `json:"rtrId,omitempty"`
	Valor   string `json:"valor"`
	Status  string `json:"status,omitempty"`
	Motivo  string `json:"motivo,omitempty"`
	Horario *struct {
		Solicitacao string `json:"solicitacao"`
	} `json:"horario,omitempty"`
	Descricao string `json:"descricao,omitempty"`
}

func caminhoDevolucao(e2e, id string) string {
	return "/v2/pix/" + url.PathEscape(e2e) + "/devolucao/" + url.PathEscape(id)
}

func paraDevolucao(conta domain.Conta, r devolucaoEfi, base domain.Devolucao) (domain.Devolucao, error) {
	d := base
	d.ContaID = conta.ID
	if r.ID != "" {
		d.ID = r.ID
	}
	if r.Valor != "" {
		v, err := domain.ParseMoney(r.Valor)
		if err != nil {
			return domain.Devolucao{}, fmt.Errorf("efi: valor da devolução %q: %w", r.Valor, err)
		}
		d.Valor = v
	}
	d.Status = domain.StatusDevolucao(r.Status)
	if r.Motivo != "" {
		d.Motivo = r.Motivo
	}
	if r.Horario != nil && r.Horario.Solicitacao != "" {
		criada, err := parseHorario(r.Horario.Solicitacao)
		if err != nil {
			return domain.Devolucao{}, fmt.Errorf("efi: horario.solicitacao: %w", err)
		}
		d.CriadaEm = criada
	}
	return d, nil
}

// Devolver pede a devolução (PUT /v2/pix/:e2eId/devolucao/:id).
//
// O Efí recusa id repetido com 409 devolucao_id_duplicado. O port pede
// idempotência pelo id, então o adapter consulta a devolução existente e a
// devolve se o valor for o mesmo; valor diferente é ErrJaExiste.
func (p *PSP) Devolver(ctx context.Context, conta domain.Conta, d domain.Devolucao) (domain.Devolucao, error) {
	corpo := devolucaoEfi{Valor: d.Valor.String(), Descricao: d.Motivo}
	var r devolucaoEfi
	err := p.chamar(ctx, conta, http.MethodPut, caminhoDevolucao(d.EndToEndID, d.ID), corpo, &r)
	if errors.Is(err, errDevolucaoDuplicada) {
		var existente devolucaoEfi
		if errGet := p.chamar(ctx, conta, http.MethodGet, caminhoDevolucao(d.EndToEndID, d.ID), nil, &existente); errGet != nil {
			return domain.Devolucao{}, err
		}
		if existente.Valor != corpo.Valor {
			return domain.Devolucao{}, fmt.Errorf("efi: devolução %s já existe com valor %s: %w", d.ID, existente.Valor, ports.ErrJaExiste)
		}
		return paraDevolucao(conta, existente, d)
	}
	if err != nil {
		return domain.Devolucao{}, err
	}
	return paraDevolucao(conta, r, d)
}
