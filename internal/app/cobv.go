package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// CobrancasV são os casos de uso da cobrança com vencimento (cobv).
type CobrancasV struct {
	Store ports.Store
	PSPs  ports.Registry
	Clock ports.Clock
}

// PedidoModalidade é multa, juros ou abatimento como chega da API.
type PedidoModalidade struct {
	Modalidade int
	ValorPerc  string
}

// PedidoDescontoData é um desconto até uma data.
type PedidoDescontoData struct {
	Data, ValorPerc string
}

// PedidoDesconto é o desconto como chega da API.
type PedidoDesconto struct {
	Modalidade int
	DatasFixas []PedidoDescontoData
	ValorPerc  *string
}

// PedidoDevedorCompleto é o devedor da cobv como chega da API.
type PedidoDevedorCompleto struct {
	PedidoDevedor
	Email, Logradouro, Cidade, UF, CEP string
}

// PedidoCobv é a criação de uma cobv (PUT /cobv/{txid}).
type PedidoCobv struct {
	Txid               string
	Chave              string
	Valor              string
	Vencimento         string
	ValidadeDias       *int
	Devedor            *PedidoDevedorCompleto
	Multa, Juros       *PedidoModalidade
	Abatimento         *PedidoModalidade
	Desconto           *PedidoDesconto
	SolicitacaoPagador string
}

// RevisaoCobv é um PATCH: só os campos não nulos mudam. Multa, juros,
// abatimento e desconto, quando vêm, substituem o anterior inteiro.
type RevisaoCobv struct {
	Chave              *string
	Valor              *string
	Vencimento         *string
	ValidadeDias       *int
	Devedor            *PedidoDevedorCompleto
	Multa, Juros       *PedidoModalidade
	Abatimento         *PedidoModalidade
	Desconto           *PedidoDesconto
	SolicitacaoPagador *string
	Remover            bool
}

// Criar valida a cobv inteira, encontra a conta pela chave, cria no PSP e grava
// com o evento cobranca.criada.
func (s CobrancasV) Criar(ctx context.Context, p PedidoCobv) (domain.CobrancaVencimento, error) {
	var v validacao
	c := domain.CobrancaVencimento{Status: domain.CobrancaAtiva, ValidadeAposVencimento: domain.ValidadeAposVencimentoPadrao, SolicitacaoPagador: p.SolicitacaoPagador}
	if tx, err := domain.ParseTxid(p.Txid); err != nil {
		v.add("txid", err.Error(), p.Txid)
	} else {
		c.Txid = tx
	}
	if p.Chave == "" {
		v.add("chave", "obrigatória", "")
	}
	if p.Devedor == nil {
		v.add("devedor", "obrigatório na cobrança com vencimento", "")
	}
	hoje := domain.DataDe(s.Clock.Now())
	aplicarCamposCobv(&v, &c, camposCobv{
		valor: &p.Valor, vencimento: &p.Vencimento, validade: p.ValidadeDias, devedor: p.Devedor,
		multa: p.Multa, juros: p.Juros, abatimento: p.Abatimento, desconto: p.Desconto, solicitacao: &p.SolicitacaoPagador,
	}, hoje)
	if err := v.err(); err != nil {
		return domain.CobrancaVencimento{}, err
	}

	conta, err := s.Store.Contas().BuscarPorChave(ctx, p.Chave)
	if err == nil && !AcessoDe(ctx).enxerga(conta.ID) {
		err = ports.ErrNaoEncontrado
	}
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return domain.CobrancaVencimento{}, &ErroValidacao{Violacoes: []Violacao{{Propriedade: "chave", Razao: "não pertence a nenhuma conta desta instância", Valor: p.Chave}}}
	}
	if err != nil {
		return domain.CobrancaVencimento{}, err
	}
	if err := conta.PodeCobrar(); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapCobv)
	if err != nil {
		return domain.CobrancaVencimento{}, err
	}
	if err := s.txidLivre(ctx, c.Txid); err != nil {
		return domain.CobrancaVencimento{}, err
	}

	c.ContaID, c.Chave, c.CriadaEm = conta.ID, conta.ChavePix, s.Clock.Now()
	criada, err := psp.CriarCobv(ctx, conta, c)
	if err != nil {
		return domain.CobrancaVencimento{}, erroDoPSP(err)
	}
	c.Location, c.BRCode = criada.Location, criada.BRCode
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.CobrancasV().Criar(ctx, c); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(domain.EventoCobrancaCriada, c))
	})
	if err != nil {
		return domain.CobrancaVencimento{}, err
	}
	return s.Store.CobrancasV().BuscarPorTxid(ctx, c.Txid)
}

// txidLivre confere cob e cobv antes de chamar o banco (RN-10).
func (s CobrancasV) txidLivre(ctx context.Context, txid domain.Txid) error {
	if _, err := s.Store.Cobrancas().BuscarPorTxid(ctx, txid); err == nil {
		return fmt.Errorf("cobrança %s: %w", txid, ports.ErrJaExiste)
	} else if !errors.Is(err, ports.ErrNaoEncontrado) {
		return err
	}
	if _, err := s.Store.CobrancasV().BuscarPorTxid(ctx, txid); err == nil {
		return fmt.Errorf("cobrança %s: %w", txid, ports.ErrJaExiste)
	} else if !errors.Is(err, ports.ErrNaoEncontrado) {
		return err
	}
	return nil
}

// Consultar devolve a cobv, sincronizando com o PSP se ela ainda estiver ativa.
func (s CobrancasV) Consultar(ctx context.Context, txid string) (domain.CobrancaVencimento, error) {
	tx, err := domain.ParseTxid(txid)
	if err != nil {
		return domain.CobrancaVencimento{}, &ErroValidacao{Violacoes: []Violacao{{Propriedade: "txid", Razao: err.Error(), Valor: txid}}}
	}
	c, err := s.Store.CobrancasV().BuscarPorTxid(ctx, tx)
	if err == nil && !AcessoDe(ctx).enxerga(c.ContaID) {
		return domain.CobrancaVencimento{}, fmt.Errorf("cobrança %s: %w", tx, ports.ErrNaoEncontrado)
	}
	if err != nil || c.Status != domain.CobrancaAtiva {
		return c, err
	}
	return s.sincronizar(ctx, c)
}

func (s CobrancasV) sincronizar(ctx context.Context, c domain.CobrancaVencimento) (domain.CobrancaVencimento, error) {
	conta, err := s.Store.Contas().Buscar(ctx, c.ContaID)
	if err != nil {
		return c, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapCobv)
	if err != nil {
		return c, nil //nolint:nilerr // sem PSP para perguntar, o estado local é a melhor resposta
	}
	remota, err := psp.ConsultarCobv(ctx, conta, c.Txid)
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
		if err := tx.CobrancasV().Atualizar(ctx, &c); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(tipo, c))
	})
	if errors.Is(err, ports.ErrConflito) {
		return s.Store.CobrancasV().BuscarPorTxid(ctx, c.Txid)
	}
	return c, err
}

// Revisar aplica um PATCH, revalidando a cobv inteira resultante.
func (s CobrancasV) Revisar(ctx context.Context, txid string, r RevisaoCobv) (domain.CobrancaVencimento, error) {
	atual, err := s.Consultar(ctx, txid)
	if err != nil {
		return domain.CobrancaVencimento{}, err
	}
	if atual.Status != domain.CobrancaAtiva {
		return domain.CobrancaVencimento{}, fmt.Errorf("cobrança %s está %s: %w", atual.Txid, atual.Status, domain.ErrTransicaoInvalida)
	}
	var v validacao
	nova := atual
	if r.Chave != nil && *r.Chave != atual.Chave {
		v.add("chave", "não pode mudar: a chave define a conta da cobrança", *r.Chave)
	}
	hoje := domain.DataDe(s.Clock.Now())
	aplicarCamposCobv(&v, &nova, camposCobv{
		valor: r.Valor, vencimento: r.Vencimento, validade: r.ValidadeDias, devedor: r.Devedor,
		multa: r.Multa, juros: r.Juros, abatimento: r.Abatimento, desconto: r.Desconto, solicitacao: r.SolicitacaoPagador,
	}, hoje)
	if err := v.err(); err != nil {
		return domain.CobrancaVencimento{}, err
	}
	tipo := domain.EventoCobrancaRevisada
	if r.Remover {
		if err := nova.Transitar(domain.CobrancaRemovidaPeloUsuarioRecebedor); err != nil {
			return domain.CobrancaVencimento{}, err
		}
		tipo = domain.EventoCobrancaRemovida
	}
	conta, err := s.Store.Contas().Buscar(ctx, atual.ContaID)
	if err != nil {
		return domain.CobrancaVencimento{}, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapCobv)
	if err != nil {
		return domain.CobrancaVencimento{}, err
	}
	revisada, err := psp.RevisarCobv(ctx, conta, nova)
	if err != nil {
		return domain.CobrancaVencimento{}, erroDoPSP(err)
	}
	nova.BRCode = revisada.BRCode
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.CobrancasV().Atualizar(ctx, &nova); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(tipo, nova))
	})
	return nova, err
}

// PaginaCobvs é o resultado de Listar.
type PaginaCobvs struct {
	Cobs         []domain.CobrancaVencimento
	Pagina       int
	ItensPagina  int
	TotalPaginas int
	TotalItens   int
}

// Listar devolve uma página das cobvs criadas em [Inicio, Fim).
func (s CobrancasV) Listar(ctx context.Context, q ConsultaCobs) (PaginaCobvs, error) {
	if err := validarConsulta(&q); err != nil {
		return PaginaCobvs{}, err
	}
	f := ports.FiltroCobrancas{ContaID: AcessoDe(ctx).ContaID, Status: q.Status, Documento: q.Documento, Janela: ports.Janela{Inicio: q.Inicio, Fim: q.Fim}}
	total, err := s.Store.CobrancasV().Contar(ctx, f)
	if err != nil {
		return PaginaCobvs{}, err
	}
	f.Limite, f.Pular = q.ItensPagina, q.Pagina*q.ItensPagina
	cobs, _, err := s.Store.CobrancasV().Listar(ctx, f)
	if err != nil {
		return PaginaCobvs{}, err
	}
	return PaginaCobvs{Cobs: cobs, Pagina: q.Pagina, ItensPagina: q.ItensPagina, TotalPaginas: totalPaginas(total, q.ItensPagina), TotalItens: total}, nil
}

func (s CobrancasV) evento(tipo domain.TipoEvento, c domain.CobrancaVencimento) domain.Evento {
	return domain.Evento{
		ID: novoID(), Tipo: tipo, ContaID: c.ContaID, OcorridoEm: s.Clock.Now(),
		Dados: DadosCobranca{Tipo: "cobv", Txid: string(c.Txid), ContaID: c.ContaID, Status: string(c.Status), Valor: c.Valor.String(), Revisao: c.Revisao},
	}
}

// camposCobv são os campos opcionais que criação e revisão aplicam do mesmo jeito.
type camposCobv struct {
	valor, vencimento, solicitacao *string
	validade                       *int
	devedor                        *PedidoDevedorCompleto
	multa, juros, abatimento       *PedidoModalidade
	desconto                       *PedidoDesconto
}

// aplicarCamposCobv aplica os campos presentes e revalida as combinações (RN-14).
func aplicarCamposCobv(v *validacao, c *domain.CobrancaVencimento, f camposCobv, hoje domain.Data) {
	if f.valor != nil {
		if m, err := domain.ParseMoney(*f.valor); err != nil || !m.Valid() {
			v.add("valor.original", "deve ser um valor positivo no formato 10.50", *f.valor)
		} else {
			c.Valor = m
		}
	}
	if f.vencimento != nil {
		if d, err := domain.ParseData(*f.vencimento); err != nil {
			v.add("calendario.dataDeVencimento", err.Error(), *f.vencimento)
		} else if d.Antes(hoje) {
			v.add("calendario.dataDeVencimento", "não pode estar no passado", *f.vencimento)
		} else {
			c.Vencimento = d
		}
	}
	if f.validade != nil {
		if *f.validade < 0 {
			v.add("calendario.validadeAposVencimento", "não pode ser negativa", fmt.Sprint(*f.validade))
		} else {
			c.ValidadeAposVencimento = *f.validade
		}
	}
	if f.devedor != nil {
		aplicarDevedorCompleto(v, c, f.devedor)
	}
	if f.solicitacao != nil {
		if len([]rune(*f.solicitacao)) > 140 {
			v.add("solicitacaoPagador", "até 140 caracteres", "")
		} else {
			c.SolicitacaoPagador = *f.solicitacao
		}
	}
	if f.multa != nil {
		c.Multa = modalidade(v, "valor.multa", f.multa, []int{1, 2}, 2)
	}
	if f.juros != nil {
		c.Juros = modalidade(v, "valor.juros", f.juros, []int{1, 2, 3, 4, 5, 6, 7, 8}, 0)
	}
	if f.abatimento != nil {
		c.Abatimento = modalidade(v, "valor.abatimento", f.abatimento, []int{1, 2}, 2)
	}
	if f.desconto != nil {
		c.Desconto = desconto(v, f.desconto)
	}

	// Combinações que dependem de mais de um campo, revalidadas no estado final.
	if c.Abatimento != nil && c.Abatimento.Modalidade == 1 && c.Valor != 0 && c.Abatimento.ValorPerc >= c.Valor {
		v.add("valor.abatimento.valorPerc", "abatimento fixo deve ser menor que o valor original", c.Abatimento.ValorPerc.String())
	}
	if d := c.Desconto; d != nil {
		for i, df := range d.DatasFixas {
			if !c.Vencimento.Zero() && df.Data.Depois(c.Vencimento) {
				v.add(fmt.Sprintf("valor.desconto.descontoDataFixa[%d].data", i), "deve ser até a data de vencimento", df.Data.String())
			}
			if d.Modalidade == 1 && c.Valor != 0 && df.ValorPerc >= c.Valor {
				v.add(fmt.Sprintf("valor.desconto.descontoDataFixa[%d].valorPerc", i), "desconto fixo deve ser menor que o valor original", df.ValorPerc.String())
			}
		}
	}
}

// modalidade valida multa, juros ou abatimento. percentual é a modalidade em que
// ValorPerc é percentual, que então não pode passar de 100%; 0 dispensa o teto.
func modalidade(v *validacao, prop string, p *PedidoModalidade, validas []int, percentual int) *domain.ValorModalidade {
	ok := true
	if !slices.Contains(validas, p.Modalidade) {
		v.add(prop+".modalidade", fmt.Sprintf("modalidade %d inválida (aceitas: %v)", p.Modalidade, validas), fmt.Sprint(p.Modalidade))
		ok = false
	}
	m, err := domain.ParseMoney(p.ValorPerc)
	if err != nil || !m.Valid() {
		v.add(prop+".valorPerc", "deve ser positivo no formato 2.00", p.ValorPerc)
		ok = false
	} else if percentual != 0 && p.Modalidade == percentual && m > 10000 {
		v.add(prop+".valorPerc", "percentual não pode passar de 100.00", p.ValorPerc)
		ok = false
	}
	if !ok {
		return nil
	}
	return &domain.ValorModalidade{Modalidade: p.Modalidade, ValorPerc: m}
}

// desconto valida as duas famílias: datas fixas (1 e 2) ou antecipação (3 a 6).
func desconto(v *validacao, p *PedidoDesconto) *domain.Desconto {
	const prop = "valor.desconto"
	d := &domain.Desconto{Modalidade: p.Modalidade}
	switch p.Modalidade {
	case 1, 2:
		if p.ValorPerc != nil {
			v.add(prop+".valorPerc", "modalidades 1 e 2 usam descontoDataFixa, não valorPerc", *p.ValorPerc)
		}
		if len(p.DatasFixas) < 1 || len(p.DatasFixas) > 3 {
			v.add(prop+".descontoDataFixa", "informe de 1 a 3 datas", fmt.Sprint(len(p.DatasFixas)))
		}
		vistas := map[string]bool{}
		for i, df := range p.DatasFixas {
			campo := fmt.Sprintf("%s.descontoDataFixa[%d]", prop, i)
			data, err := domain.ParseData(df.Data)
			if err != nil {
				v.add(campo+".data", err.Error(), df.Data)
				continue
			}
			if vistas[df.Data] {
				v.add(campo+".data", "data repetida", df.Data)
			}
			vistas[df.Data] = true
			m, err := domain.ParseMoney(df.ValorPerc)
			if err != nil || !m.Valid() || (p.Modalidade == 2 && m >= 10000) {
				v.add(campo+".valorPerc", "deve ser positivo (e menor que 100.00 se percentual)", df.ValorPerc)
				continue
			}
			d.DatasFixas = append(d.DatasFixas, domain.DescontoData{Data: data, ValorPerc: m})
		}
	case 3, 4, 5, 6:
		if len(p.DatasFixas) > 0 {
			v.add(prop+".descontoDataFixa", "modalidades 3 a 6 usam valorPerc, não descontoDataFixa", "")
		}
		if p.ValorPerc == nil {
			v.add(prop+".valorPerc", "obrigatório nas modalidades 3 a 6", "")
			return nil
		}
		m, err := domain.ParseMoney(*p.ValorPerc)
		if err != nil || !m.Valid() || ((p.Modalidade == 5 || p.Modalidade == 6) && m >= 10000) {
			v.add(prop+".valorPerc", "deve ser positivo (e menor que 100.00 se percentual)", *p.ValorPerc)
			return nil
		}
		d.ValorPerc = m
	default:
		v.add(prop+".modalidade", fmt.Sprintf("modalidade %d inválida (aceitas: 1 a 6)", p.Modalidade), fmt.Sprint(p.Modalidade))
		return nil
	}
	return d
}

func aplicarDevedorCompleto(v *validacao, c *domain.CobrancaVencimento, d *PedidoDevedorCompleto) {
	var parcial domain.Cobranca
	aplicarDevedor(v, &parcial, &d.PedidoDevedor)
	if parcial.Devedor == nil {
		return
	}
	if d.UF != "" && (len(d.UF) != 2 || d.UF != upperASCII(d.UF)) {
		v.add("devedor.uf", "sigla de 2 letras maiúsculas", d.UF)
	}
	if d.CEP != "" && (len(d.CEP) != 8 || !soDigitosASCII(d.CEP)) {
		v.add("devedor.cep", "8 dígitos, sem traço", d.CEP)
	}
	c.Devedor = domain.DevedorCompleto{Devedor: *parcial.Devedor, Email: d.Email, Logradouro: d.Logradouro, Cidade: d.Cidade, UF: d.UF, CEP: d.CEP}
}

func upperASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

func soDigitosASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
