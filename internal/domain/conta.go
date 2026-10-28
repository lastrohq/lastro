package domain

// Ambiente diz se a conta fala com a homologação ou com a produção do banco.
type Ambiente string

// Ambientes possíveis de uma conta.
const (
	Homologacao Ambiente = "homologacao"
	Producao    Ambiente = "producao"
)

// Conta é uma conta recebedora: um PSP, uma chave Pix e as credenciais do
// dono (RN-03). As credenciais ficam no Vault, nunca nesta struct.
type Conta struct {
	ID       string
	Nome     string
	PSP      string
	ChavePix string
	Ambiente Ambiente
	Ativa    bool
}
