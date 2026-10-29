package domain

import "time"

// TipoEvento nomeia o que aconteceu, no formato entidade.acao.
type TipoEvento string

// Eventos publicados para a aplicação do cliente (RN-32).
const (
	EventoCobrancaCriada      TipoEvento = "cobranca.criada"
	EventoCobrancaConcluida   TipoEvento = "cobranca.concluida"
	EventoCobrancaRevisada    TipoEvento = "cobranca.revisada"
	EventoCobrancaRemovida    TipoEvento = "cobranca.removida"
	EventoPixRecebido         TipoEvento = "pix.recebido"
	EventoDevolucaoAtualizada TipoEvento = "devolucao.atualizada"
	EventoCertificadoVencendo TipoEvento = "certificado.expirando"
	// Pix Automático (RN-40, RN-41).
	EventoRecorrenciaCriada      TipoEvento = "recorrencia.criada"
	EventoRecorrenciaAtualizada  TipoEvento = "recorrencia.atualizada"
	EventoSolicitacaoRecorrencia TipoEvento = "recorrencia.solicitacao"
)

// Evento é uma mudança de estado que vira webhook de saída. Dados é o payload
// normalizado, independente do banco.
type Evento struct {
	ID         string
	Tipo       TipoEvento
	ContaID    string
	OcorridoEm time.Time
	Dados      any
}
