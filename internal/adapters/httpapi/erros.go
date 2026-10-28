package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

func tipoErro(nome string) string { return "https://lastro.dev/erros/" + nome }

func ptr[T any](v T) *T { return &v }

// problemaDe traduz os erros do núcleo para o formato da spec do BCB. É o único
// lugar que decide status HTTP de erro.
func problemaDe(ctx context.Context, err error) gen.Problema {
	p := func(status int, tipo, titulo string) gen.Problema {
		return gen.Problema{Status: status, Type: tipoErro(tipo), Title: titulo, Detail: ptr(err.Error())}
	}
	var ev *app.ErroValidacao
	switch {
	case errors.As(err, &ev):
		pr := p(http.StatusBadRequest, "RequisicaoInvalida", "A requisição viola o schema ou uma regra de negócio")
		vs := make([]gen.Violacao, len(ev.Violacoes))
		for i, v := range ev.Violacoes {
			vs[i] = gen.Violacao{Propriedade: ptr(v.Propriedade), Razao: ptr(v.Razao), Valor: ptr(v.Valor)}
		}
		pr.Violacoes = &vs
		return pr
	case errors.Is(err, ports.ErrNaoEncontrado):
		return p(http.StatusNotFound, "CobNaoEncontrado", "Cobrança não encontrada")
	case errors.Is(err, ports.ErrJaExiste):
		return p(http.StatusConflict, "CobOperacaoInvalida", "Já existe uma cobrança com esse txid")
	case errors.Is(err, ports.ErrConflito):
		return p(http.StatusConflict, "CobOperacaoInvalida", "A cobrança mudou durante a operação; tente de novo")
	case errors.Is(err, domain.ErrTransicaoInvalida):
		return p(http.StatusBadRequest, "CobOperacaoInvalida", "A cobrança não está num estado que permita essa operação")
	case errors.Is(err, domain.ErrContaInativa):
		return p(http.StatusBadRequest, "ContaInativa", "A conta da chave informada está desativada")
	case errors.Is(err, ports.ErrNaoSuportado):
		return p(http.StatusNotImplemented, "NaoSuportado", "O PSP da conta não oferece essa operação")
	case errors.Is(err, errNaoImplementado):
		return p(http.StatusNotImplemented, "NaoImplementado", "Ainda não implementado")
	case errors.Is(err, app.ErrPSP):
		return p(http.StatusBadGateway, "PSPIndisponivel", "O PSP recusou ou falhou")
	}
	slog.ErrorContext(ctx, "erro interno", "erro", err)
	// Erro inesperado não vaza detalhe para o cliente.
	return gen.Problema{Status: http.StatusInternalServerError, Type: tipoErro("ErroInterno"), Title: "Erro interno"}
}
