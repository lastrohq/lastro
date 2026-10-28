package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Cobrancas são os casos de uso da cobrança imediata (cob).
type Cobrancas struct {
	Store ports.Store
	PSPs  ports.Registry
	Clock ports.Clock
}

// PedidoDevedor é o devedor como chega da API: CPF ou CNPJ, nunca os dois.
type PedidoDevedor struct {
	CPF, CNPJ, Nome string
}

// PedidoCob é a criação de uma cob, com os campos ainda no formato da spec.
type PedidoCob struct {
	// Txid vazio faz o Lastro gerar um.
	Txid               string
	Chave              string
	Valor              string
	PermiteAlteracao   bool
	ExpiracaoSegundos  *int
	Devedor            *PedidoDevedor
	SolicitacaoPagador string
}

// RevisaoCob é um PATCH: só os campos não nulos mudam.
type RevisaoCob struct {
	Chave              *string
	Valor              *string
	PermiteAlteracao   *bool
	ExpiracaoSegundos  *int
	Devedor            *PedidoDevedor
	SolicitacaoPagador *string
	// Remover leva a cobrança a REMOVIDA_PELO_USUARIO_RECEBEDOR.
	Remover bool
}

// DadosCobranca é o payload normalizado dos eventos de cobrança (RN-32).
type DadosCobranca struct {
	Txid    string `json:"txid"`
	ContaID string `json:"contaId"`
	Status  string `json:"status"`
	Valor   string `json:"valor"`
	Revisao int    `json:"revisao"`
}

// Criar valida o pedido, encontra a conta pela chave, cria no PSP e grava a
// cobrança com o evento cobranca.criada na mesma transação.
func (s Cobrancas) Criar(ctx context.Context, p PedidoCob) (domain.Cobranca, error) {
	var v validacao
	c := domain.Cobranca{Status: domain.CobrancaAtiva, PermiteAlteracao: p.PermiteAlteracao, SolicitacaoPagador: p.SolicitacaoPagador, Expiracao: domain.ExpiracaoPadrao}

	if p.Txid == "" {
		c.Txid = domain.NewTxid()
	} else if tx, err := domain.ParseTxid(p.Txid); err != nil {
		v.add("txid", err.Error(), p.Txid)
	} else {
		c.Txid = tx
	}
	aplicarValor(&v, &c, p.Valor)
	aplicarExpiracao(&v, &c, p.ExpiracaoSegundos)
	aplicarDevedor(&v, &c, p.Devedor)
	aplicarSolicitacao(&v, &c, p.SolicitacaoPagador)
	if p.Chave == "" {
		v.add("chave", "obrigatória", "")
	}
	if err := v.err(); err != nil {
		return domain.Cobranca{}, err
	}

	conta, err := s.Store.Contas().BuscarPorChave(ctx, p.Chave)
	if err == nil && !AcessoDe(ctx).enxerga(conta.ID) {
		// Para quem não enxerga a conta, a chave é tão desconhecida quanto uma que não existe.
		err = ports.ErrNaoEncontrado
	}
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return domain.Cobranca{}, &ErroValidacao{Violacoes: []Violacao{{Propriedade: "chave", Razao: "não pertence a nenhuma conta desta instância", Valor: p.Chave}}}
	}
	if err != nil {
		return domain.Cobranca{}, err
	}
	if err := conta.PodeCobrar(); err != nil {
		return domain.Cobranca{}, err
	}
	psp, err := s.psp(conta, ports.CapCob)
	if err != nil {
		return domain.Cobranca{}, err
	}
	// Conferir antes de chamar o banco: txid repetido não pode virar cobrança órfã no PSP.
	if _, err := s.Store.Cobrancas().BuscarPorTxid(ctx, c.Txid); err == nil {
		return domain.Cobranca{}, fmt.Errorf("cobrança %s: %w", c.Txid, ports.ErrJaExiste)
	} else if !errors.Is(err, ports.ErrNaoEncontrado) {
		return domain.Cobranca{}, err
	}

	c.ContaID, c.Chave, c.CriadaEm = conta.ID, conta.ChavePix, s.Clock.Now()
	criada, err := psp.CriarCob(ctx, conta, c)
	if err != nil {
		return domain.Cobranca{}, erroDoPSP(err)
	}
	c.Location, c.BRCode = criada.Location, criada.BRCode

	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Cobrancas().Criar(ctx, c); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(domain.EventoCobrancaCriada, c))
	})
	if err != nil {
		return domain.Cobranca{}, err
	}
	return s.Store.Cobrancas().Buscar(ctx, c.ContaID, c.Txid)
}

// Consultar devolve a cobrança. Se ela ainda está ATIVA, pergunta ao PSP e,
// se ele já a concluiu, grava a conclusão e publica cobranca.concluida.
// Falha do PSP não impede a consulta: devolve o estado local.
func (s Cobrancas) Consultar(ctx context.Context, txid string) (domain.Cobranca, error) {
	tx, err := domain.ParseTxid(txid)
	if err != nil {
		return domain.Cobranca{}, &ErroValidacao{Violacoes: []Violacao{{Propriedade: "txid", Razao: err.Error(), Valor: txid}}}
	}
	c, err := s.Store.Cobrancas().BuscarPorTxid(ctx, tx)
	if err == nil && !AcessoDe(ctx).enxerga(c.ContaID) {
		return domain.Cobranca{}, fmt.Errorf("cobrança %s: %w", tx, ports.ErrNaoEncontrado)
	}
	if err != nil || c.Status != domain.CobrancaAtiva {
		return c, err
	}
	return s.sincronizar(ctx, c)
}

func (s Cobrancas) sincronizar(ctx context.Context, c domain.Cobranca) (domain.Cobranca, error) {
	conta, err := s.Store.Contas().Buscar(ctx, c.ContaID)
	if err != nil {
		return c, err
	}
	psp, err := s.psp(conta, ports.CapCob)
	if err != nil {
		return c, nil //nolint:nilerr // sem PSP para perguntar, o estado local é a melhor resposta
	}
	remota, err := psp.ConsultarCob(ctx, conta, c.Txid)
	if err != nil {
		slog.WarnContext(ctx, "consulta ao PSP falhou; devolvendo estado local", "txid", c.Txid, "psp", conta.PSP, "erro", err)
		return c, nil
	}
	if remota.Status == c.Status || !domain.TransicoesCobranca.Pode(c.Status, remota.Status) {
		return c, nil
	}
	_ = c.Transitar(remota.Status)
	tipo := domain.EventoCobrancaConcluida
	if remota.Status != domain.CobrancaConcluida {
		tipo = domain.EventoCobrancaRemovida
	}
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Cobrancas().Atualizar(ctx, &c); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(tipo, c))
	})
	if errors.Is(err, ports.ErrConflito) {
		// Outra requisição gravou primeiro; a versão dela é a que vale.
		return s.Store.Cobrancas().Buscar(ctx, c.ContaID, c.Txid)
	}
	return c, err
}

// Revisar aplica um PATCH. Primeiro sincroniza com o PSP, para não revisar
// localmente uma cobrança que o banco já concluiu.
func (s Cobrancas) Revisar(ctx context.Context, txid string, r RevisaoCob) (domain.Cobranca, error) {
	atual, err := s.Consultar(ctx, txid)
	if err != nil {
		return domain.Cobranca{}, err
	}
	if atual.Status != domain.CobrancaAtiva {
		return domain.Cobranca{}, fmt.Errorf("cobrança %s está %s: %w", atual.Txid, atual.Status, domain.ErrTransicaoInvalida)
	}
	conta, err := s.Store.Contas().Buscar(ctx, atual.ContaID)
	if err != nil {
		return domain.Cobranca{}, err
	}

	var v validacao
	nova := atual
	if r.Chave != nil && *r.Chave != atual.Chave {
		v.add("chave", "não pode mudar: a chave define a conta da cobrança", *r.Chave)
	}
	if r.Valor != nil {
		aplicarValor(&v, &nova, *r.Valor)
	}
	if r.PermiteAlteracao != nil {
		nova.PermiteAlteracao = *r.PermiteAlteracao
	}
	if r.ExpiracaoSegundos != nil {
		aplicarExpiracao(&v, &nova, r.ExpiracaoSegundos)
	}
	if r.Devedor != nil {
		aplicarDevedor(&v, &nova, r.Devedor)
	}
	if r.SolicitacaoPagador != nil {
		aplicarSolicitacao(&v, &nova, *r.SolicitacaoPagador)
	}
	if err := v.err(); err != nil {
		return domain.Cobranca{}, err
	}
	tipo := domain.EventoCobrancaRevisada
	if r.Remover {
		if err := nova.Transitar(domain.CobrancaRemovidaPeloUsuarioRecebedor); err != nil {
			return domain.Cobranca{}, err
		}
		tipo = domain.EventoCobrancaRemovida
	}

	psp, err := s.psp(conta, ports.CapCob)
	if err != nil {
		return domain.Cobranca{}, err
	}
	revisada, err := psp.RevisarCob(ctx, conta, nova)
	if err != nil {
		return domain.Cobranca{}, erroDoPSP(err)
	}
	nova.BRCode = revisada.BRCode

	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.Cobrancas().Atualizar(ctx, &nova); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(tipo, nova))
	})
	return nova, err
}

// ConsultaCobs são os filtros do GET /cob, com a paginação por página da spec.
type ConsultaCobs struct {
	Inicio, Fim time.Time
	Documento   string
	Status      domain.StatusCobranca
	Pagina      int
	ItensPagina int
}

// PaginaCobs é o resultado de Listar.
type PaginaCobs struct {
	Cobs         []domain.Cobranca
	Pagina       int
	ItensPagina  int
	TotalPaginas int
	TotalItens   int
}

// Listar devolve uma página das cobranças criadas em [Inicio, Fim).
func (s Cobrancas) Listar(ctx context.Context, q ConsultaCobs) (PaginaCobs, error) {
	var v validacao
	if q.Inicio.IsZero() || q.Fim.IsZero() || !q.Inicio.Before(q.Fim) {
		v.add("inicio", "inicio e fim são obrigatórios, com inicio antes de fim", "")
	}
	if q.ItensPagina == 0 {
		q.ItensPagina = 100
	}
	if q.ItensPagina < 1 || q.ItensPagina > 1000 {
		v.add("paginacao.itensPorPagina", "deve estar entre 1 e 1000", fmt.Sprint(q.ItensPagina))
	}
	if q.Pagina < 0 {
		v.add("paginacao.paginaAtual", "não pode ser negativa", fmt.Sprint(q.Pagina))
	}
	if err := v.err(); err != nil {
		return PaginaCobs{}, err
	}
	f := ports.FiltroCobrancas{ContaID: AcessoDe(ctx).ContaID, Status: q.Status, Documento: q.Documento, Janela: ports.Janela{Inicio: q.Inicio, Fim: q.Fim}}
	total, err := s.Store.Cobrancas().Contar(ctx, f)
	if err != nil {
		return PaginaCobs{}, err
	}
	f.Limite, f.Pular = q.ItensPagina, q.Pagina*q.ItensPagina
	cobs, _, err := s.Store.Cobrancas().Listar(ctx, f)
	if err != nil {
		return PaginaCobs{}, err
	}
	paginas := (total + q.ItensPagina - 1) / q.ItensPagina
	if paginas == 0 {
		paginas = 1 // a spec exige quantidadeDePaginas >= 1
	}
	return PaginaCobs{Cobs: cobs, Pagina: q.Pagina, ItensPagina: q.ItensPagina, TotalPaginas: paginas, TotalItens: total}, nil
}

func (s Cobrancas) psp(conta domain.Conta, c ports.Capacidade) (ports.PSP, error) {
	psp, err := s.PSPs.PSP(conta.PSP)
	if err != nil {
		return nil, err
	}
	if err := psp.Capabilities().Suporta(c); err != nil {
		return nil, err
	}
	return psp, nil
}

func (s Cobrancas) evento(tipo domain.TipoEvento, c domain.Cobranca) domain.Evento {
	return domain.Evento{
		ID: novoID(), Tipo: tipo, ContaID: c.ContaID, OcorridoEm: s.Clock.Now(),
		Dados: DadosCobranca{Txid: string(c.Txid), ContaID: c.ContaID, Status: string(c.Status), Valor: c.Valor.String(), Revisao: c.Revisao},
	}
}

// erroDoPSP preserva os erros conhecidos do port e embrulha o resto em ErrPSP.
func erroDoPSP(err error) error {
	for _, conhecido := range []error{ports.ErrJaExiste, ports.ErrNaoEncontrado, ports.ErrNaoSuportado, domain.ErrTransicaoInvalida} {
		if errors.Is(err, conhecido) {
			return err
		}
	}
	return fmt.Errorf("%w: %w", ErrPSP, err)
}

func aplicarValor(v *validacao, c *domain.Cobranca, valor string) {
	m, err := domain.ParseMoney(valor)
	switch {
	case err != nil:
		v.add("valor.original", err.Error(), valor)
	case !m.Valid():
		v.add("valor.original", "deve ser maior que zero", valor)
	default:
		c.Valor = m
	}
}

func aplicarExpiracao(v *validacao, c *domain.Cobranca, segundos *int) {
	if segundos == nil {
		return
	}
	if *segundos < 1 {
		v.add("calendario.expiracao", "deve ser de pelo menos 1 segundo", fmt.Sprint(*segundos))
		return
	}
	c.Expiracao = time.Duration(*segundos) * time.Second
}

func aplicarDevedor(v *validacao, c *domain.Cobranca, d *PedidoDevedor) {
	if d == nil {
		return
	}
	doc, prop := d.CPF, "devedor.cpf"
	switch {
	case d.CPF != "" && d.CNPJ != "":
		v.add("devedor", "informe cpf ou cnpj, não os dois", "")
		return
	case d.CNPJ != "":
		doc, prop = d.CNPJ, "devedor.cnpj"
	case d.CPF == "":
		v.add("devedor", "informe cpf ou cnpj", "")
		return
	}
	if err := domain.ValidarDocumento(doc); err != nil || (prop == "devedor.cpf") != (len(doc) == 11) {
		v.add(prop, "documento inválido", doc)
	}
	if d.Nome == "" || len(d.Nome) > 200 {
		v.add("devedor.nome", "obrigatório, até 200 caracteres", "")
	}
	c.Devedor = &domain.Devedor{Documento: doc, Nome: d.Nome}
}

func aplicarSolicitacao(v *validacao, c *domain.Cobranca, s string) {
	if len([]rune(s)) > 140 {
		v.add("solicitacaoPagador", "até 140 caracteres", "")
		return
	}
	c.SolicitacaoPagador = s
}
