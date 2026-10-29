package domain

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// ErrRecorrenciaInvalida indica dados de recorrência fora das regras da RN-40.
var ErrRecorrenciaInvalida = errors.New("recorrência inválida")

// Periodicidade de uma recorrência de Pix Automático (API Pix 2.10.0).
type Periodicidade string

// Periodicidades da spec do BCB.
const (
	Semanal    Periodicidade = "SEMANAL"
	Mensal     Periodicidade = "MENSAL"
	Trimestral Periodicidade = "TRIMESTRAL"
	Semestral  Periodicidade = "SEMESTRAL"
	Anual      Periodicidade = "ANUAL"
)

func (p Periodicidade) valida() bool {
	switch p {
	case Semanal, Mensal, Trimestral, Semestral, Anual:
		return true
	}
	return false
}

// PoliticaRetentativa diz se a cobrança recorrente pode ser tentada de novo
// depois do vencimento. Só o recebedor define (RN-42).
type PoliticaRetentativa string

// Políticas da spec do BCB.
const (
	SemRetentativa PoliticaRetentativa = "NAO_PERMITE"
	// Retentativa3Em7Dias permite até 3 novas datas em até 7 dias corridos.
	Retentativa3Em7Dias PoliticaRetentativa = "PERMITE_3R_7D"
)

// idRec segue a regra de formação da spec: R + (R|N) + ISPB (8 alfanuméricos) +
// data AAAAMMDD + 11 alfanuméricos. Quem gera é o PSP do recebedor.
var idRec = regexp.MustCompile(`^R[RN][0-9A-Z]{8}[0-9]{8}[a-zA-Z0-9]{11}$`)

// ValidarIDRecorrencia confere o formato do idRec.
func ValidarIDRecorrencia(id string) error {
	if !idRec.MatchString(id) {
		return fmt.Errorf("%w: idRec %q fora da regra de formação (29 caracteres, começando por RR ou RN)", ErrRecorrenciaInvalida, id)
	}
	return nil
}

// Recorrencia é a autorização de Pix Automático de um pagador para um
// recebedor (RN-40). O idRec vem do PSP depois da criação.
type Recorrencia struct {
	ID      string
	ContaID string
	Status  StatusRecorrencia

	// Vínculo: o contrato que justifica a recorrência.
	Contrato string // obrigatório, até 35 caracteres
	Objeto   string // opcional, até 35 caracteres
	Devedor  Devedor

	DataInicial   Data
	DataFinal     Data // zero: sem fim definido
	Periodicidade Periodicidade

	// ValorFixo é o valor de cada cobrança; zero quando o valor varia por ciclo.
	ValorFixo Money
	// PisoValorMaximo vale quando o valor varia: o pagador define um valor
	// máximo na autorização, e este é o menor máximo que o recebedor aceita
	// (valorMinimoRecebedor na spec; "piso para o valor máximo" no Guia v1.3).
	PisoValorMaximo Money

	Politica PoliticaRetentativa
	// MotivoRejeicao vem do pagador quando ele rejeita (RN-40).
	MotivoRejeicao string

	CriadaEm time.Time
	Revisao  int
}

// Validar confere a recorrência antes de enviá-la ao PSP.
func (r Recorrencia) Validar() error {
	falha := func(format string, args ...any) error {
		return fmt.Errorf("%w: "+format, append([]any{ErrRecorrenciaInvalida}, args...)...)
	}
	switch {
	case r.Contrato == "" || len(r.Contrato) > 35:
		return falha("contrato é obrigatório, até 35 caracteres")
	case len(r.Objeto) > 35:
		return falha("objeto tem até 35 caracteres")
	case r.Devedor.Nome == "" || len(r.Devedor.Nome) > 140:
		return falha("nome do devedor é obrigatório, até 140 caracteres")
	case r.DataInicial.Zero():
		return falha("dataInicial é obrigatória")
	case !r.DataFinal.Zero() && r.DataFinal.Antes(r.DataInicial):
		return falha("dataFinal (%s) antes da dataInicial (%s)", r.DataFinal, r.DataInicial)
	case !r.Periodicidade.valida():
		return falha("periodicidade %q (use SEMANAL, MENSAL, TRIMESTRAL, SEMESTRAL ou ANUAL)", r.Periodicidade)
	case r.Politica != SemRetentativa && r.Politica != Retentativa3Em7Dias:
		return falha("politicaRetentativa %q (use NAO_PERMITE ou PERMITE_3R_7D)", r.Politica)
	case r.ValorFixo < 0 || r.PisoValorMaximo < 0:
		return falha("valores não podem ser negativos")
	case r.ValorFixo > 0 && r.PisoValorMaximo > 0:
		return falha("valor fixo e piso do valor máximo são excludentes: o piso só vale quando o valor varia")
	}
	if err := ValidarDocumento(r.Devedor.Documento); err != nil {
		return fmt.Errorf("%w: devedor: %w", ErrRecorrenciaInvalida, err)
	}
	return nil
}

// PermiteRetentativa diz se as cobranças desta recorrência podem ser tentadas
// de novo depois do vencimento (RN-42).
func (r Recorrencia) PermiteRetentativa() bool { return r.Politica == Retentativa3Em7Dias }

// Transitar muda o status respeitando a RN-40.
func (r *Recorrencia) Transitar(para StatusRecorrencia) error {
	if err := TransicoesRecorrencia.Validar(r.Status, para); err != nil {
		return err
	}
	r.Status = para
	return nil
}

// StatusSolicitacao é o estado da solicitação de confirmação (RN-41).
type StatusSolicitacao string

// Estados da spec do BCB (SolicRecStatus, 2.10.0).
const (
	SolicitacaoCriada    StatusSolicitacao = "CRIADA"
	SolicitacaoEnviada   StatusSolicitacao = "ENVIADA"
	SolicitacaoRecebida  StatusSolicitacao = "RECEBIDA"
	SolicitacaoAceita    StatusSolicitacao = "ACEITA"
	SolicitacaoRejeitada StatusSolicitacao = "REJEITADA"
	SolicitacaoExpirada  StatusSolicitacao = "EXPIRADA"
	SolicitacaoCancelada StatusSolicitacao = "CANCELADA"
)

// TransicoesSolicitacao segue a RN-41. A spec lista os status, não as
// transições: esta tabela segue a ordem dos nomes e precisa ser conferida no
// Manual de Fluxos do BCB. Expirar e cancelar valem enquanto não houve resposta.
var TransicoesSolicitacao = Transicoes[StatusSolicitacao]{
	SolicitacaoCriada:   {SolicitacaoEnviada, SolicitacaoExpirada, SolicitacaoCancelada},
	SolicitacaoEnviada:  {SolicitacaoRecebida, SolicitacaoExpirada, SolicitacaoCancelada},
	SolicitacaoRecebida: {SolicitacaoAceita, SolicitacaoRejeitada, SolicitacaoExpirada, SolicitacaoCancelada},
}

// PendenciaMaximaSolicitacao é quanto a solicitação pode ficar no app do
// pagador antes de expirar (Guia do Pix Automático v1.3, seção 3.2; RN-43).
const PendenciaMaximaSolicitacao = 30 * 24 * time.Hour

// ContaPagador é a conta do pagador para onde vai a solicitação (jornada 1).
type ContaPagador struct {
	ISPB    string // do participante do pagador, 8 caracteres
	Agencia string // até 4
	Conta   string // até 20
}

// SolicitacaoRecorrencia é o pedido de confirmação enviado ao app do pagador (RN-41).
type SolicitacaoRecorrencia struct {
	ID            string // idSolicRec, vem do PSP
	RecorrenciaID string
	ContaID       string
	Status        StatusSolicitacao
	Destinatario  ContaPagador
	ExpiraEm      time.Time
	CriadaEm      time.Time
}

var ispb = regexp.MustCompile(`^[0-9A-Z]{8}$`)

// Validar confere a solicitação no instante agora: a expiração tem que estar
// no futuro e dentro da pendência máxima.
func (s SolicitacaoRecorrencia) Validar(agora time.Time) error {
	falha := func(format string, args ...any) error {
		return fmt.Errorf("%w: solicitação: "+format, append([]any{ErrRecorrenciaInvalida}, args...)...)
	}
	switch {
	case s.RecorrenciaID == "":
		return falha("idRec é obrigatório")
	case !ispb.MatchString(s.Destinatario.ISPB):
		return falha("ispbParticipante %q deve ter 8 caracteres", s.Destinatario.ISPB)
	case s.Destinatario.Conta == "" || len(s.Destinatario.Conta) > 20:
		return falha("conta do pagador é obrigatória, até 20 caracteres")
	case len(s.Destinatario.Agencia) > 4:
		return falha("agência tem até 4 caracteres")
	case !s.ExpiraEm.After(agora):
		return falha("dataExpiracaoSolicitacao precisa estar no futuro")
	case s.ExpiraEm.Sub(agora) > PendenciaMaximaSolicitacao:
		return falha("dataExpiracaoSolicitacao passa da pendência máxima de 30 dias")
	}
	return nil
}

// Transitar muda o status respeitando a RN-41.
func (s *SolicitacaoRecorrencia) Transitar(para StatusSolicitacao) error {
	if err := TransicoesSolicitacao.Validar(s.Status, para); err != nil {
		return err
	}
	s.Status = para
	return nil
}
