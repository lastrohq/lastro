package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// CobrancasRecorrentes cria, cancela e pede retentativa das cobranças de uma
// recorrência aprovada (RN-42). As mudanças que chegam do banco ficam para o
// webhook de cobrança recorrente.
type CobrancasRecorrentes struct {
	Store ports.Store
	PSPs  ports.Registry
	Clock ports.Clock
}

// DadosCobrancaRecorrente é o payload de cobranca_recorrente.criada,
// cobranca_recorrente.atualizada e cobranca_recorrente.tentativa.
type DadosCobrancaRecorrente struct {
	Txid       string `json:"txid"`
	IDRec      string `json:"idRec"`
	ContaID    string `json:"contaId"`
	Status     string `json:"status"`
	Valor      string `json:"valor"`
	Vencimento string `json:"vencimento"`
	Revisao    int    `json:"revisao"`
	// Tentativa é a data de liquidação que o evento anuncia (só em .tentativa).
	Tentativa *DadosTentativa `json:"tentativa,omitempty"`
	// MotivoEncerramento vem na rejeição e no cancelamento.
	MotivoEncerramento string `json:"motivoEncerramento,omitempty"`
}

// DadosTentativa é uma tentativa de liquidação no payload dos eventos.
type DadosTentativa struct {
	DataLiquidacao string `json:"dataLiquidacao"`
	Tipo           string `json:"tipo"`
	Status         string `json:"status"`
}

func (s CobrancasRecorrentes) evento(tipo domain.TipoEvento, c domain.CobrancaRecorrente, t *domain.Tentativa) domain.Evento {
	d := DadosCobrancaRecorrente{
		Txid: string(c.Txid), IDRec: c.RecorrenciaID, ContaID: c.ContaID, Status: string(c.Status),
		Valor: c.Valor.String(), Vencimento: c.Vencimento.String(), Revisao: c.Revisao, MotivoEncerramento: c.MotivoEncerramento,
	}
	if t != nil {
		d.Tentativa = &DadosTentativa{DataLiquidacao: t.DataLiquidacao.String(), Tipo: string(t.Tipo), Status: string(t.Status)}
	}
	return domain.Evento{ID: novoID(), Tipo: tipo, ContaID: c.ContaID, OcorridoEm: s.Clock.Now(), Dados: d}
}

// pspCobr encontra o adapter da conta e confere que ele faz cobrança
// recorrente: declarar CapCobr e implementar ports.PSPCobrancaRecorrente.
func pspCobr(r ports.Registry, conta domain.Conta) (ports.PSPCobrancaRecorrente, error) {
	psp, err := pspDe(r, conta, ports.CapCobr)
	if err != nil {
		return nil, err
	}
	cobr, ok := psp.(ports.PSPCobrancaRecorrente)
	if !ok {
		return nil, fmt.Errorf("%w: o adapter %s declara cobr mas não implementa a cobrança recorrente", ports.ErrNaoSuportado, conta.PSP)
	}
	return cobr, nil
}

// txidLivre confere, antes de chamar o banco, que o txid não é de nenhuma
// cobrança da instância (RN-10): txid repetido não pode virar cobrança órfã no PSP.
func txidLivre(ctx context.Context, st ports.Store, txid domain.Txid) error {
	buscas := []func() error{
		func() error { _, err := st.Cobrancas().BuscarPorTxid(ctx, txid); return err },
		func() error { _, err := st.CobrancasV().BuscarPorTxid(ctx, txid); return err },
		func() error { _, err := st.CobrancasRecorrentes().BuscarPorTxid(ctx, txid); return err },
	}
	for _, buscar := range buscas {
		err := buscar()
		if err == nil {
			return fmt.Errorf("cobrança %s: %w", txid, ports.ErrJaExiste)
		}
		if !errors.Is(err, ports.ErrNaoEncontrado) {
			return err
		}
	}
	return nil
}

// Criar valida a cobrança contra a recorrência (RN-42), envia ao PSP da conta
// e grava o que ele devolveu, com o evento cobranca_recorrente.criada. Txid
// vazio faz o Lastro gerar um. Cada ciclo tem uma cobrança só: outra no mesmo
// ciclo só entra se a anterior foi cancelada, rejeitada ou expirou.
func (s CobrancasRecorrentes) Criar(ctx context.Context, idRec string, c domain.CobrancaRecorrente) (domain.CobrancaRecorrente, error) {
	rec, err := Recorrencias{Store: s.Store}.Consultar(ctx, idRec)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	conta, err := s.Store.Contas().Buscar(ctx, rec.ContaID)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if err := conta.PodeCobrar(); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if c.Txid == "" {
		c.Txid = domain.NewTxid()
	} else if _, err := domain.ParseTxid(string(c.Txid)); err != nil {
		return domain.CobrancaRecorrente{}, &ErroValidacao{Violacoes: []Violacao{{Propriedade: "txid", Razao: err.Error(), Valor: string(c.Txid)}}}
	}
	c.RecorrenciaID, c.ContaID, c.Status, c.Revisao = rec.ID, conta.ID, domain.CobrCriada, 0
	c.Tentativas, c.MotivoEncerramento, c.CriadaEm = nil, "", s.Clock.Now()
	if err := c.ValidarCriacao(rec, s.Clock.Now()); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if err := s.cicloLivre(ctx, rec, c.Vencimento); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	psp, err := pspCobr(s.PSPs, conta)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if err := txidLivre(ctx, s.Store, c.Txid); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	criada, err := psp.CriarCobr(ctx, conta, c)
	if err != nil {
		return domain.CobrancaRecorrente{}, erroDoPSP(err)
	}
	if criada.Txid != c.Txid {
		return domain.CobrancaRecorrente{}, fmt.Errorf("%w: o PSP devolveu o txid %q para a cobrança %s", ErrPSP, criada.Txid, c.Txid)
	}
	if criada.Status != "" {
		c.Status = criada.Status
	}
	c.Tentativas = criada.Tentativas
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := tx.CobrancasRecorrentes().Criar(ctx, c); err != nil {
			return err
		}
		return tx.Outbox().Adicionar(ctx, s.evento(domain.EventoCobrRCriada, c, nil))
	})
	if err != nil {
		return domain.CobrancaRecorrente{}, fmt.Errorf("o PSP criou a cobrança recorrente %s, mas ela não foi gravada: %w", c.Txid, err)
	}
	return c, nil
}

// cicloLivre recusa uma segunda cobrança no ciclo do vencimento enquanto a
// que já existe pode ser paga ou já foi.
func (s CobrancasRecorrentes) cicloLivre(ctx context.Context, rec domain.Recorrencia, venc domain.Data) error {
	existentes, err := s.Store.CobrancasRecorrentes().DaRecorrencia(ctx, rec.ID)
	if err != nil {
		return err
	}
	ciclo := rec.InicioDoCicloSeguinte(venc)
	for _, e := range existentes {
		if rec.InicioDoCicloSeguinte(e.Vencimento) != ciclo {
			continue
		}
		switch e.Status {
		case domain.CobrCriada, domain.CobrAtiva, domain.CobrConcluida:
			return fmt.Errorf("%w: o ciclo de %s já tem a cobrança %s (%s)", domain.ErrCobrancaRecorrenteInvalida, venc, e.Txid, e.Status)
		}
	}
	return nil
}

// Consultar devolve a cobrança gravada. Acesso restrito a outra conta não a
// enxerga: responde ErrNaoEncontrado, sem revelar que ela existe.
func (s CobrancasRecorrentes) Consultar(ctx context.Context, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	c, err := s.Store.CobrancasRecorrentes().BuscarPorTxid(ctx, txid)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if !AcessoDe(ctx).enxerga(c.ContaID) {
		return domain.CobrancaRecorrente{}, fmt.Errorf("cobrança recorrente %s: %w", txid, ports.ErrNaoEncontrado)
	}
	return c, nil
}

// Cancelar leva a cobrança a CANCELADA no PSP e grava, com o evento
// cobranca_recorrente.atualizada. O recebedor só cancela (RN-42).
func (s CobrancasRecorrentes) Cancelar(ctx context.Context, txid domain.Txid) (domain.CobrancaRecorrente, error) {
	c, err := s.Consultar(ctx, txid)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if err := domain.TransicoesCobrancaRecorrente.Validar(c.Status, domain.CobrCancelada); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	conta, err := s.Store.Contas().Buscar(ctx, c.ContaID)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	psp, err := pspCobr(s.PSPs, conta)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	noBanco, err := psp.CancelarCobr(ctx, conta, c.Txid)
	if err != nil {
		return domain.CobrancaRecorrente{}, erroDoPSP(err)
	}
	return s.gravar(ctx, c.Txid, func(atual *domain.CobrancaRecorrente) (*domain.Tentativa, error) {
		if atual.Status == domain.CobrCancelada {
			return nil, errSemMudanca // o aviso do banco chegou antes
		}
		if err := atual.Transitar(domain.CobrCancelada); err != nil {
			return nil, err
		}
		atual.MotivoEncerramento = noBanco.MotivoEncerramento
		return nil, nil
	})
}

// SolicitarRetentativa pede uma nova data de liquidação para a cobrança que
// não foi paga no vencimento (RN-42) e publica cobranca_recorrente.tentativa.
func (s CobrancasRecorrentes) SolicitarRetentativa(ctx context.Context, txid domain.Txid, data domain.Data) (domain.CobrancaRecorrente, error) {
	c, err := s.Consultar(ctx, txid)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	rec, err := s.Store.Recorrencias().Buscar(ctx, c.RecorrenciaID)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	if err := c.ValidarRetentativa(rec, data, s.Clock.Now()); err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	conta, err := s.Store.Contas().Buscar(ctx, c.ContaID)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	psp, err := pspCobr(s.PSPs, conta)
	if err != nil {
		return domain.CobrancaRecorrente{}, err
	}
	noBanco, err := psp.SolicitarRetentativa(ctx, conta, c.Txid, data)
	if err != nil {
		return domain.CobrancaRecorrente{}, erroDoPSP(err)
	}
	// O banco é a fonte da verdade sobre a tentativa; se ele não a devolveu,
	// ela entra como pedida.
	nova := domain.Tentativa{DataLiquidacao: data, Tipo: domain.TipoNovaTentativa, Status: domain.TentativaSolicitada}
	for _, t := range noBanco.Tentativas {
		if t.DataLiquidacao == data && t.Tipo != domain.TipoAgendamento {
			nova = t
		}
	}
	return s.gravar(ctx, c.Txid, func(atual *domain.CobrancaRecorrente) (*domain.Tentativa, error) {
		for i, t := range atual.Tentativas {
			if t.DataLiquidacao == data && t.Tipo != domain.TipoAgendamento {
				atual.Tentativas[i] = nova // o aviso do banco chegou antes
				return &nova, nil
			}
		}
		atual.Tentativas = append(atual.Tentativas, nova)
		return &nova, nil
	})
}

// errSemMudanca é a mudança que o banco já tinha avisado: gravar não grava nem publica.
var errSemMudanca = errors.New("sem mudança")

// gravar relê a cobrança na transação, aplica a mudança e publica o evento:
// cobranca_recorrente.tentativa se a mudança devolve uma tentativa,
// cobranca_recorrente.atualizada se não. Reler, em vez de usar a revisão de
// antes da chamada ao PSP, deixa passar o aviso do banco que chegou no meio.
func (s CobrancasRecorrentes) gravar(ctx context.Context, txid domain.Txid, mudar func(*domain.CobrancaRecorrente) (*domain.Tentativa, error)) (domain.CobrancaRecorrente, error) {
	var c domain.CobrancaRecorrente
	err := s.Store.WithTx(ctx, func(tx ports.Store) error {
		var err error
		if c, err = tx.CobrancasRecorrentes().BuscarPorTxid(ctx, txid); err != nil {
			return err
		}
		t, err := mudar(&c)
		if errors.Is(err, errSemMudanca) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.CobrancasRecorrentes().Atualizar(ctx, &c); err != nil {
			return err
		}
		tipo := domain.EventoCobrRAtualizada
		if t != nil {
			tipo = domain.EventoCobrRTentativa
		}
		return tx.Outbox().Adicionar(ctx, s.evento(tipo, c, t))
	})
	if err != nil {
		return domain.CobrancaRecorrente{}, fmt.Errorf("o PSP aceitou a mudança na cobrança recorrente %s, mas ela não foi gravada: %w", txid, err)
	}
	return c, nil
}
