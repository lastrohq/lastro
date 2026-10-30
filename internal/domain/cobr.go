package domain

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// ErrCobrancaRecorrenteInvalida indica uma cobrança recorrente fora da RN-42.
var ErrCobrancaRecorrenteInvalida = errors.New("cobrança recorrente inválida")

// ErrRetentativaRecusada indica um pedido de retentativa fora da RN-42.
var ErrRetentativaRecusada = errors.New("retentativa recusada")

// StatusCobrancaRecorrente é o estado de uma cobrança recorrente (RN-42).
type StatusCobrancaRecorrente string

// Estados da spec do BCB (CobRStatus, 2.10.0).
const (
	CobrCriada    StatusCobrancaRecorrente = "CRIADA"
	CobrAtiva     StatusCobrancaRecorrente = "ATIVA"
	CobrConcluida StatusCobrancaRecorrente = "CONCLUIDA"
	CobrExpirada  StatusCobrancaRecorrente = "EXPIRADA"
	CobrRejeitada StatusCobrancaRecorrente = "REJEITADA"
	CobrCancelada StatusCobrancaRecorrente = "CANCELADA"
)

// TransicoesCobrancaRecorrente segue a RN-42, conferida nos diagramas de
// estados da CobR do Manual de Padrões para Iniciação do Pix 2.10.0 (seção 3.3
// do Pix Automático): CRIADA é a cobrança registrada no PSP do recebedor e
// ATIVA, a enviada ao PSP do pagador para agendamento, que continua ATIVA
// durante as retentativas. A rejeição pelo PSP do pagador passa por ATIVA.
var TransicoesCobrancaRecorrente = Transicoes[StatusCobrancaRecorrente]{
	CobrCriada: {CobrAtiva, CobrCancelada},
	CobrAtiva:  {CobrConcluida, CobrExpirada, CobrRejeitada, CobrCancelada},
}

// TipoTentativa diz de onde veio a tentativa de liquidação.
type TipoTentativa string

// Tipos da spec do BCB. A spec não descreve os tipos; o exemplo dela mostra
// AGND na data original e NTAG na data nova de uma retentativa.
const (
	TipoAgendamento   TipoTentativa = "AGND"
	TipoNovaTentativa TipoTentativa = "NTAG"
	TipoRIFL          TipoTentativa = "RIFL"
)

// StatusTentativa é o estado de uma tentativa de liquidação.
type StatusTentativa string

// Estados da spec do BCB.
const (
	TentativaSolicitada StatusTentativa = "SOLICITADA"
	TentativaAgendada   StatusTentativa = "AGENDADA"
	TentativaPaga       StatusTentativa = "PAGA"
	TentativaCancelada  StatusTentativa = "CANCELADA"
	TentativaRejeitada  StatusTentativa = "REJEITADA"
	TentativaExpirada   StatusTentativa = "EXPIRADA"
)

// Tentativa é uma data em que o PSP do pagador tenta liquidar a cobrança.
type Tentativa struct {
	DataLiquidacao Data
	Tipo           TipoTentativa
	Status         StatusTentativa
	EndToEndID     string
	// MotivoRejeicao vem do PSP do pagador: código e descrição.
	MotivoRejeicao string
}

// CobrancaRecorrente é a cobrança de um ciclo de uma recorrência aprovada (cobr).
type CobrancaRecorrente struct {
	Txid          Txid
	RecorrenciaID string // idRec
	ContaID       string
	Status        StatusCobrancaRecorrente
	Valor         Money

	// Vencimento é a data prevista de liquidação (dataDeVencimento).
	Vencimento Data
	// AjusteDiaUtil leva a liquidação para o próximo dia útil quando o
	// vencimento cai num dia não útil. O default da spec é true.
	AjusteDiaUtil bool
	InfoAdicional string // até 140 caracteres

	Tentativas []Tentativa
	// MotivoEncerramento vem quando a cobrança é rejeitada ou cancelada.
	MotivoEncerramento string

	CriadaEm time.Time
	Revisao  int
}

// ValidarCriacao confere a cobrança contra a recorrência no instante agora,
// antes de enviá-la ao PSP (RN-42): recorrência aprovada, vencimento dentro da
// vigência e da janela de antecedência que vale hoje (RN-43), valor da recorrência.
func (c CobrancaRecorrente) ValidarCriacao(rec Recorrencia, agora time.Time) error {
	falha := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrCobrancaRecorrenteInvalida}, args...)...)
	}
	hoje := DataDe(agora)
	p := ParametrosEm(hoje)
	primeiro, ultimo := hoje.SomarDias(p.AntecedenciaMinimaDias), hoje.SomarDias(p.AntecedenciaMaximaDias)
	switch {
	case c.RecorrenciaID != rec.ID:
		return falha("idRec %q não é o da recorrência %q", c.RecorrenciaID, rec.ID)
	case rec.Status != RecorrenciaAprovada:
		return falha("recorrência %s está %s; só gera cobrança quando APROVADA", rec.ID, rec.Status)
	case !c.Valor.Valid():
		return falha("valor %s fora do intervalo aceito", c.Valor)
	case rec.ValorFixo > 0 && c.Valor != rec.ValorFixo:
		return falha("valor %s diferente do valor fixo da recorrência (%s)", c.Valor, rec.ValorFixo)
	case len(c.InfoAdicional) > 140:
		return falha("infoAdicional tem até 140 caracteres")
	case c.Vencimento.Zero():
		return falha("dataDeVencimento é obrigatória")
	case c.Vencimento.Antes(rec.DataInicial):
		return falha("dataDeVencimento (%s) antes do início da recorrência (%s)", c.Vencimento, rec.DataInicial)
	case !rec.DataFinal.Zero() && c.Vencimento.Depois(rec.DataFinal):
		return falha("dataDeVencimento (%s) depois do fim da recorrência (%s)", c.Vencimento, rec.DataFinal)
	case c.Vencimento.Antes(primeiro) || c.Vencimento.Depois(ultimo):
		return falha("dataDeVencimento (%s) fora da antecedência de %d a %d dias corridos: hoje vai de %s a %s",
			c.Vencimento, p.AntecedenciaMinimaDias, p.AntecedenciaMaximaDias, primeiro, ultimo)
	}
	return nil
}

// Retentativas devolve as datas novas já pedidas depois do vencimento, sem repetir.
func (c CobrancaRecorrente) Retentativas() []Data {
	var datas []Data
	vistas := map[Data]bool{}
	for _, t := range c.Tentativas {
		if t.Tipo == TipoAgendamento || vistas[t.DataLiquidacao] {
			continue
		}
		vistas[t.DataLiquidacao] = true
		datas = append(datas, t.DataLiquidacao)
	}
	return datas
}

// ValidarRetentativa confere o pedido de uma nova data de liquidação no
// instante agora (RN-42). O valor não entra: a retentativa é sempre do mesmo
// valor, e o pedido nem o carrega.
func (c CobrancaRecorrente) ValidarRetentativa(rec Recorrencia, nova Data, agora time.Time) error {
	falha := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrRetentativaRecusada}, args...)...)
	}
	p := ParametrosEm(DataDe(agora))
	limite := c.Vencimento.SomarDias(p.JanelaRetentativaDias)
	feitas := c.Retentativas()
	switch {
	case c.RecorrenciaID != rec.ID:
		return falha("idRec %q não é o da recorrência %q", c.RecorrenciaID, rec.ID)
	case !rec.PermiteRetentativa():
		return falha("a recorrência %s tem política %s", rec.ID, rec.Politica)
	case c.Status != CobrAtiva:
		return falha("cobrança está %s; só ATIVA ganha nova data", c.Status)
	case !nova.Depois(c.Vencimento):
		return falha("a data nova (%s) tem que vir depois do vencimento (%s)", nova, c.Vencimento)
	case nova.Depois(limite):
		return falha("a data nova (%s) passa de %d dias corridos do vencimento (até %s)", nova, p.JanelaRetentativaDias, limite)
	case !nova.Antes(rec.InicioDoCicloSeguinte(c.Vencimento)):
		return falha("a data nova (%s) cai no ciclo seguinte, que começa em %s", nova, rec.InicioDoCicloSeguinte(c.Vencimento))
	case !rec.DataFinal.Zero() && nova.Depois(rec.DataFinal):
		return falha("a data nova (%s) passa do fim da recorrência (%s)", nova, rec.DataFinal)
	case !DataDe(agora).Antes(nova):
		return falha("a data nova (%s) tem que ser pedida até 23h59 do dia anterior", nova)
	}
	for _, d := range feitas {
		if d == nova {
			return falha("a data %s já foi pedida", nova)
		}
	}
	if len(feitas) >= p.RetentativasMaximas {
		return falha("a cobrança já teve as %d datas novas permitidas", p.RetentativasMaximas)
	}
	return nil
}

// Transitar muda o status respeitando a RN-42.
func (c *CobrancaRecorrente) Transitar(para StatusCobrancaRecorrente) error {
	if err := TransicoesCobrancaRecorrente.Validar(c.Status, para); err != nil {
		return err
	}
	c.Status = para
	return nil
}

// Somar devolve o início do n-ésimo ciclo contado a partir de d. Nos ciclos
// por mês, o dia que não existe no mês vira o último dia dele: uma recorrência
// mensal que começa em 31/01 cobra em 28/02 (ou 29/02) e volta a 31/03.
func (p Periodicidade) Somar(d Data, n int) Data {
	if p == Semanal {
		return d.SomarDias(7 * n)
	}
	meses := map[Periodicidade]int{Mensal: 1, Trimestral: 3, Semestral: 6, Anual: 12}[p] * n
	ano, mes, dia := d.t.Date()
	primeiroDoMes := time.Date(ano, mes+time.Month(meses), 1, 0, 0, 0, 0, time.UTC)
	ultimoDia := primeiroDoMes.AddDate(0, 1, -1).Day()
	return NovaData(primeiroDoMes.Year(), primeiroDoMes.Month(), min(dia, ultimoDia))
}

// InicioDoCicloSeguinte devolve o início do primeiro ciclo da recorrência
// depois da data d. Periodicidade fora da spec devolve a data zero.
func (r Recorrencia) InicioDoCicloSeguinte(d Data) Data {
	if !r.Periodicidade.valida() {
		return Data{}
	}
	for n := 1; ; n++ {
		if inicio := r.Periodicidade.Somar(r.DataInicial, n); inicio.Depois(d) {
			return inicio
		}
	}
}

// AtualizacaoCobrancaRecorrente é a cobrança recorrente segundo o banco, no
// webhook de cobrança recorrente ou na consulta (RN-42).
type AtualizacaoCobrancaRecorrente struct {
	Txid   Txid
	IDRec  string
	Status StatusCobrancaRecorrente
	// Tentativas é o histórico que o banco conhece; zero tentativas não apaga nada.
	Tentativas []Tentativa
	// MotivoEncerramento vem na rejeição e no cancelamento.
	MotivoEncerramento string
}

// Aplicar leva para a cobrança o que o banco avisou e devolve se o status
// mudou e quais tentativas são novas ou mudaram. O banco é a fonte da verdade
// sobre cada tentativa, identificada pela data e pelo tipo; tentativa que só o
// Vintém conhece (pedida e ainda não avisada) continua. Status fora da RN-42,
// como uma volta de CONCLUIDA para ATIVA, é ErrTransicaoInvalida, e nada muda.
func (c *CobrancaRecorrente) Aplicar(a AtualizacaoCobrancaRecorrente) (statusMudou bool, tentativas []Tentativa, err error) {
	novo := *c
	novo.Tentativas = append([]Tentativa(nil), c.Tentativas...)
	if a.Status != "" && a.Status != c.Status {
		// O status vem do banco: vale se for alcançável, porque o aviso do
		// passo intermediário (CRIADA → ATIVA) pode ter se perdido.
		if err := TransicoesCobrancaRecorrente.ValidarCaminho(c.Status, a.Status); err != nil {
			return false, nil, err
		}
		novo.Status = a.Status
		statusMudou = true
	}
	if a.MotivoEncerramento != "" && a.MotivoEncerramento != c.MotivoEncerramento {
		novo.MotivoEncerramento = a.MotivoEncerramento
	}
	for _, t := range a.Tentativas {
		i := slices.IndexFunc(novo.Tentativas, func(x Tentativa) bool { return x.DataLiquidacao == t.DataLiquidacao && x.Tipo == t.Tipo })
		switch {
		case i < 0:
			novo.Tentativas = append(novo.Tentativas, t)
		case novo.Tentativas[i] == t:
			continue
		default:
			novo.Tentativas[i] = t
		}
		tentativas = append(tentativas, t)
	}
	*c = novo
	return statusMudou, tentativas, nil
}
