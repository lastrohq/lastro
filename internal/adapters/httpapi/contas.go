package httpapi

import (
	"context"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/domain"
)

// CriarConta cadastra uma conta recebedora com as credenciais cifradas.
func (s *Server) CriarConta(ctx context.Context, r gen.CriarContaRequestObject) (gen.CriarContaResponseObject, error) {
	b := r.Body
	c := domain.Conta{ID: b.Id, Nome: b.Nome, PSP: b.Psp, ChavePix: b.Chave, Ambiente: domain.Ambiente(b.Ambiente)}
	var cred domain.Credenciais
	if b.Credenciais != nil {
		cred = domain.Credenciais{ClientID: deref(b.Credenciais.ClientId), ClientSecret: deref(b.Credenciais.ClientSecret), SenhaCertificado: deref(b.Credenciais.SenhaCertificado), ChaveAplicacao: deref(b.Credenciais.ChaveAplicacao)}
		if b.Credenciais.Certificado != nil {
			cred.Certificado = *b.Credenciais.Certificado
		}
	}
	if err := s.contas.Criar(ctx, c, cred); err != nil {
		return nil, err
	}
	criada, err := s.contas.Buscar(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	return gen.CriarConta201JSONResponse(contaGen(criada)), nil
}

// ListarContas lista as contas, sem credenciais.
func (s *Server) ListarContas(ctx context.Context, _ gen.ListarContasRequestObject) (gen.ListarContasResponseObject, error) {
	lista, err := s.contas.Listar(ctx)
	if err != nil {
		return nil, err
	}
	out := gen.ListarContas200JSONResponse{Contas: make([]gen.Conta, len(lista))}
	for i, c := range lista {
		out.Contas[i] = contaGen(c)
	}
	return out, nil
}

// ConsultarConta devolve uma conta, sem credenciais.
func (s *Server) ConsultarConta(ctx context.Context, r gen.ConsultarContaRequestObject) (gen.ConsultarContaResponseObject, error) {
	c, err := s.contas.Buscar(ctx, r.Id)
	if err != nil {
		return nil, err
	}
	return gen.ConsultarConta200JSONResponse(contaGen(c)), nil
}

// DesativarConta impede novas cobranças na conta.
func (s *Server) DesativarConta(ctx context.Context, r gen.DesativarContaRequestObject) (gen.DesativarContaResponseObject, error) {
	if err := s.contas.Desativar(ctx, r.Id); err != nil {
		return nil, err
	}
	c, err := s.contas.Buscar(ctx, r.Id)
	if err != nil {
		return nil, err
	}
	return gen.DesativarConta200JSONResponse(contaGen(c)), nil
}

func contaGen(c domain.Conta) gen.Conta {
	out := gen.Conta{Id: c.ID, Nome: c.Nome, Psp: c.PSP, Chave: c.ChavePix, Ambiente: gen.ContaAmbiente(c.Ambiente), Ativa: c.Ativa}
	if cert := c.Certificado; cert != nil {
		out.Certificado = &gen.ContaCertificado{Sujeito: cert.Sujeito, ImpressaoDigital: cert.ImpressaoDigital, Validade: cert.Validade}
	}
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
