package domain

// StatusCobranca é o estado de uma cobrança cob ou cobv (RN-12).
type StatusCobranca string

// Estados de cobrança da spec do BCB.
const (
	CobrancaAtiva                        StatusCobranca = "ATIVA"
	CobrancaConcluida                    StatusCobranca = "CONCLUIDA"
	CobrancaRemovidaPeloUsuarioRecebedor StatusCobranca = "REMOVIDA_PELO_USUARIO_RECEBEDOR"
	CobrancaRemovidaPeloPSP              StatusCobranca = "REMOVIDA_PELO_PSP"
)

// TransicoesCobranca define a RN-12: só ATIVA sai do lugar; os outros três são terminais.
var TransicoesCobranca = Transicoes[StatusCobranca]{
	CobrancaAtiva: {CobrancaConcluida, CobrancaRemovidaPeloUsuarioRecebedor, CobrancaRemovidaPeloPSP},
}

// StatusDevolucao é o estado de uma devolução (RN-20).
type StatusDevolucao string

// Estados de devolução da spec do BCB.
const (
	DevolucaoEmProcessamento StatusDevolucao = "EM_PROCESSAMENTO"
	DevolucaoDevolvido       StatusDevolucao = "DEVOLVIDO"
	DevolucaoNaoRealizado    StatusDevolucao = "NAO_REALIZADO"
)

// TransicoesDevolucao define a RN-20: a devolução termina devolvida ou não realizada.
var TransicoesDevolucao = Transicoes[StatusDevolucao]{
	DevolucaoEmProcessamento: {DevolucaoDevolvido, DevolucaoNaoRealizado},
}

// StatusRecorrencia é o estado de uma recorrência de Pix Automático (RN-40).
type StatusRecorrencia string

// Estados de recorrência da spec do BCB.
const (
	RecorrenciaCriada    StatusRecorrencia = "CRIADA"
	RecorrenciaAprovada  StatusRecorrencia = "APROVADA"
	RecorrenciaRejeitada StatusRecorrencia = "REJEITADA"
	RecorrenciaExpirada  StatusRecorrencia = "EXPIRADA"
	RecorrenciaCancelada StatusRecorrencia = "CANCELADA"
)

// TransicoesRecorrencia define a RN-40, conferida no diagrama de estados da Rec
// do Manual de Padrões para Iniciação do Pix 2.10.0 (seção 3.1 do Pix
// Automático): criada pode ser cancelada antes da aprovação; aprovada ainda
// pode ser cancelada ou expirar no fim da vigência.
var TransicoesRecorrencia = Transicoes[StatusRecorrencia]{
	RecorrenciaCriada:   {RecorrenciaAprovada, RecorrenciaRejeitada, RecorrenciaExpirada, RecorrenciaCancelada},
	RecorrenciaAprovada: {RecorrenciaCancelada, RecorrenciaExpirada},
}
