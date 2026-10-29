package domain

import "time"

// Parametros são os prazos do Pix Automático que vêm de normativo do BCB
// (RN-43). Não são constantes no código: cada mudança do BCB vira uma linha
// nova em TabelaParametros, com a fonte e a data a partir da qual vale.
type Parametros struct {
	// Fonte é o normativo de onde os valores saíram, com versão e seção.
	Fonte string
	// Desde é a data do normativo: os valores valem a partir dela.
	Desde Data

	// PendenciaMaximaSolicitacao é quanto a solicitação de confirmação pode
	// ficar no app do pagador antes de expirar (RN-41).
	PendenciaMaximaSolicitacao time.Duration

	// AntecedenciaMinimaDias e AntecedenciaMaximaDias delimitam, em dias
	// corridos antes da data prevista de liquidação, quando a instrução de
	// pagamento da cobrança recorrente pode ser enviada (RN-42).
	AntecedenciaMinimaDias int
	AntecedenciaMaximaDias int

	// RetentativasMaximas é quantas datas novas a cobrança recorrente pode
	// ganhar depois do vencimento, e JanelaRetentativaDias, em até quantos
	// dias corridos da data original (RN-42, política PERMITE_3R_7D).
	RetentativasMaximas   int
	JanelaRetentativaDias int
}

// TabelaParametros guarda os parâmetros em ordem de Desde. Mudança do BCB
// entra como linha nova no fim, nunca editando uma linha que já vale.
var TabelaParametros = []Parametros{
	{
		Fonte: "Guia de implementação do Pix Automático v1.3 (seções 3.2, agendamento e novas tentativas)",
		Desde: NovaData(2026, time.August, 19),

		PendenciaMaximaSolicitacao: 30 * 24 * time.Hour,
		AntecedenciaMinimaDias:     2,
		AntecedenciaMaximaDias:     10,
		RetentativasMaximas:        3,
		JanelaRetentativaDias:      7,
	},
}

// ParametrosEm devolve os parâmetros que valem na data d: a última linha com
// Desde até d. Antes da primeira linha vale a primeira, que é o mais antigo
// que o Lastro conhece.
func ParametrosEm(d Data) Parametros {
	p := TabelaParametros[0]
	for _, linha := range TabelaParametros[1:] {
		if linha.Desde.Depois(d) {
			break
		}
		p = linha
	}
	return p
}
