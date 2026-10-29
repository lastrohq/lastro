package domain

import "time"

// ValidadeAposVencimentoPadrao é o default da spec para validadeAposVencimento.
const ValidadeAposVencimentoPadrao = 30

// ValorModalidade é multa, juros ou abatimento: a modalidade diz se ValorPerc
// é valor em centavos ou percentual em centésimos (1050 = 10,50%).
type ValorModalidade struct {
	Modalidade int
	ValorPerc  Money
}

// DescontoData é um desconto válido até uma data (modalidades 1 e 2).
type DescontoData struct {
	Data      Data
	ValorPerc Money
}

// Desconto da cobrança com vencimento: datas fixas (modalidades 1 e 2) ou
// por antecipação (3 a 6), nunca os dois.
type Desconto struct {
	Modalidade int
	DatasFixas []DescontoData
	ValorPerc  Money
}

// DevedorCompleto é o devedor da cobv, que exige identificação (RN-14).
type DevedorCompleto struct {
	Devedor
	Email, Logradouro, Cidade, UF, CEP string
}

// CobrancaVencimento é a cobrança com vencimento (cobv).
type CobrancaVencimento struct {
	Txid    Txid
	ContaID string
	Chave   string
	Status  StatusCobranca
	Valor   Money

	Multa      *ValorModalidade
	Juros      *ValorModalidade
	Abatimento *ValorModalidade
	Desconto   *Desconto

	Vencimento             Data
	ValidadeAposVencimento int
	Devedor                DevedorCompleto

	SolicitacaoPagador string
	Location           string
	BRCode             string
	CriadaEm           time.Time
	Revisao            int
}

// UltimoDiaParaPagar é o vencimento mais a validade após o vencimento.
func (c CobrancaVencimento) UltimoDiaParaPagar() Data {
	return c.Vencimento.SomarDias(c.ValidadeAposVencimento)
}

// Expirada informa se a cobv ativa passou do último dia para pagar.
func (c CobrancaVencimento) Expirada(agora time.Time) bool {
	return c.Status == CobrancaAtiva && DataDe(agora).Depois(c.UltimoDiaParaPagar())
}

// Transitar muda o status respeitando a mesma máquina da cob (RN-12).
func (c *CobrancaVencimento) Transitar(para StatusCobranca) error {
	if err := TransicoesCobranca.Validar(c.Status, para); err != nil {
		return err
	}
	c.Status = para
	return nil
}
