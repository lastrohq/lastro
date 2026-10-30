package httpapi

import (
	"context"
	"fmt"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// errSemRecorrencias responde 501 quando o servidor foi montado sem Pix Automático.
var errSemRecorrencias = fmt.Errorf("%w: o Pix Automático não está configurado nesta instância", ports.ErrNaoSuportado)

// CriarRec cria a recorrência na conta da API key, que precisa ser restrita:
// é ela que identifica o recebedor, como a credencial do cliente na spec do BCB.
func (s *Server) CriarRec(ctx context.Context, r gen.CriarRecRequestObject) (gen.CriarRecResponseObject, error) {
	if s.recorrencias.Store == nil {
		return nil, errSemRecorrencias
	}
	contaID := app.AcessoDe(ctx).ContaID
	if contaID == "" {
		return nil, &app.ErroValidacao{Violacoes: []app.Violacao{{
			Propriedade: "Authorization",
			Razao:       "use uma API key restrita a uma conta (vintem apikey criar --conta ...): ela identifica o recebedor da recorrência",
		}}}
	}
	rec, err := recDoPedido(*r.Body)
	if err != nil {
		return nil, err
	}
	criada, err := s.recorrencias.Criar(ctx, contaID, rec)
	if err != nil {
		return nil, err
	}
	return gen.CriarRec201JSONResponse(recGen(criada)), nil
}

// ConsultarRec devolve a recorrência, se a API key enxergar a conta dela.
func (s *Server) ConsultarRec(ctx context.Context, r gen.ConsultarRecRequestObject) (gen.ConsultarRecResponseObject, error) {
	if s.recorrencias.Store == nil {
		return nil, errSemRecorrencias
	}
	rec, err := s.recorrencias.Consultar(ctx, r.IdRec)
	if err != nil {
		return nil, err
	}
	return gen.ConsultarRec200JSONResponse(recGen(rec)), nil
}

// CriarSolicRec pede a confirmação da recorrência no app do pagador (jornada 1).
func (s *Server) CriarSolicRec(ctx context.Context, r gen.CriarSolicRecRequestObject) (gen.CriarSolicRecResponseObject, error) {
	if s.recorrencias.Store == nil {
		return nil, errSemRecorrencias
	}
	b := r.Body
	dest := domain.ContaPagador{ISPB: b.Destinatario.IspbParticipante, Conta: b.Destinatario.Conta, Agencia: deref(b.Destinatario.Agencia)}
	sol, err := s.recorrencias.SolicitarConfirmacao(ctx, b.IdRec, dest, b.Calendario.DataExpiracaoSolicitacao)
	if err != nil {
		return nil, err
	}
	out := gen.CriarSolicRec201JSONResponse{IdRec: sol.RecorrenciaID, IdSolicRec: sol.ID, Status: gen.SolicRecGeradaStatus(sol.Status)}
	out.Calendario.DataExpiracaoSolicitacao = sol.ExpiraEm
	out.Destinatario.Conta, out.Destinatario.IspbParticipante = sol.Destinatario.Conta, sol.Destinatario.ISPB
	if sol.Destinatario.Agencia != "" {
		out.Destinatario.Agencia = ptr(sol.Destinatario.Agencia)
	}
	return out, nil
}

func dataDe(d openapi_types.Date) domain.Data {
	return domain.NovaData(d.Year(), d.Month(), d.Day())
}

func recDoPedido(b gen.RecSolicitada) (domain.Recorrencia, error) {
	r := domain.Recorrencia{
		Contrato: b.Vinculo.Contrato, Objeto: deref(b.Vinculo.Objeto),
		Devedor:       domain.Devedor{Documento: deref(b.Vinculo.Devedor.Cpf) + deref(b.Vinculo.Devedor.Cnpj), Nome: b.Vinculo.Devedor.Nome},
		DataInicial:   dataDe(b.Calendario.DataInicial),
		Periodicidade: domain.Periodicidade(b.Calendario.Periodicidade),
		Politica:      domain.PoliticaRetentativa(b.PoliticaRetentativa),
	}
	if b.Vinculo.Devedor.Cpf != nil && b.Vinculo.Devedor.Cnpj != nil {
		return r, &app.ErroValidacao{Violacoes: []app.Violacao{{Propriedade: "vinculo.devedor", Razao: "informe cpf ou cnpj, não os dois"}}}
	}
	if b.Calendario.DataFinal != nil {
		r.DataFinal = dataDe(*b.Calendario.DataFinal)
	}
	if v := b.Valor; v != nil {
		var v1 []app.Violacao
		if v.ValorRec != nil {
			m, err := domain.ParseMoney(*v.ValorRec)
			if err != nil {
				v1 = append(v1, app.Violacao{Propriedade: "valor.valorRec", Razao: err.Error(), Valor: *v.ValorRec})
			}
			r.ValorFixo = m
		}
		if v.ValorMinimoRecebedor != nil {
			m, err := domain.ParseMoney(*v.ValorMinimoRecebedor)
			if err != nil {
				v1 = append(v1, app.Violacao{Propriedade: "valor.valorMinimoRecebedor", Razao: err.Error(), Valor: *v.ValorMinimoRecebedor})
			}
			r.PisoValorMaximo = m
		}
		if len(v1) > 0 {
			return r, &app.ErroValidacao{Violacoes: v1}
		}
	}
	return r, nil
}

func recGen(r domain.Recorrencia) gen.RecGerada {
	g := gen.RecGerada{
		IdRec: r.ID, Status: gen.RecGeradaStatus(r.Status), PoliticaRetentativa: gen.RecGeradaPoliticaRetentativa(r.Politica),
		Vinculo: gen.RecVinculo{Contrato: r.Contrato, Devedor: gen.Devedor{Nome: r.Devedor.Nome}},
		Calendario: gen.RecCalendario{
			DataInicial: openapi_types.Date{Time: r.DataInicial.Tempo()}, Periodicidade: gen.RecCalendarioPeriodicidade(r.Periodicidade),
		},
	}
	if r.Objeto != "" {
		g.Vinculo.Objeto = ptr(r.Objeto)
	}
	if len(r.Devedor.Documento) == 11 {
		g.Vinculo.Devedor.Cpf = ptr(r.Devedor.Documento)
	} else {
		g.Vinculo.Devedor.Cnpj = ptr(r.Devedor.Documento)
	}
	if !r.DataFinal.Zero() {
		g.Calendario.DataFinal = &openapi_types.Date{Time: r.DataFinal.Tempo()}
	}
	switch {
	case r.ValorFixo > 0:
		g.Valor = &gen.RecValor{ValorRec: ptr(r.ValorFixo.String())}
	case r.PisoValorMaximo > 0:
		g.Valor = &gen.RecValor{ValorMinimoRecebedor: ptr(r.PisoValorMaximo.String())}
	}
	if r.MotivoRejeicao != "" {
		g.MotivoRejeicao = ptr(r.MotivoRejeicao)
	}
	return g
}
