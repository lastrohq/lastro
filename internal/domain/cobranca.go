package domain

import (
	"errors"
	"fmt"
	"time"
)

// ExpiracaoPadrao é a validade de uma cob quando o cliente não informa (RN-13).
const ExpiracaoPadrao = 86400 * time.Second

// Devedor identifica quem paga. Documento é CPF (11 dígitos) ou CNPJ
// (14 caracteres, alfanumérico desde a mudança da Receita).
type Devedor struct {
	Documento string
	Nome      string
}

// Cobranca é uma cobrança imediata (cob).
type Cobranca struct {
	Txid    Txid
	ContaID string
	// Chave é a chave Pix da conta no momento da criação.
	Chave              string
	Status             StatusCobranca
	Valor              Money
	PermiteAlteracao   bool
	Expiracao          time.Duration
	Devedor            *Devedor
	SolicitacaoPagador string
	// Location e BRCode vêm do PSP depois da criação.
	Location string
	BRCode   string
	CriadaEm time.Time
	// Revisao começa em 0 (como na spec do BCB), cresce a cada atualização e
	// protege contra escrita concorrente.
	Revisao int
}

// Expirada informa se a cobrança ainda ativa passou da validade em agora.
// A spec do BCB mantém a cob ATIVA depois de expirar; este é o campo derivado (RN-13).
func (c Cobranca) Expirada(agora time.Time) bool {
	return c.Status == CobrancaAtiva && !agora.Before(c.CriadaEm.Add(c.Expiracao))
}

// Transitar muda o status respeitando a RN-12.
func (c *Cobranca) Transitar(para StatusCobranca) error {
	if err := TransicoesCobranca.Validar(c.Status, para); err != nil {
		return err
	}
	c.Status = para
	return nil
}

// PixRecebido é um pagamento liquidado na conta, identificado pelo endToEndId.
type PixRecebido struct {
	EndToEndID  string
	ContaID     string
	Txid        Txid
	Valor       Money
	Chave       string
	Horario     time.Time
	InfoPagador string
	Pagador     *Devedor
}

// ValidarEndToEndID confere o formato do SPI: 32 caracteres alfanuméricos.
func ValidarEndToEndID(e2e string) error {
	if len(e2e) != 32 {
		return fmt.Errorf("%w: endToEndId deve ter 32 caracteres, tem %d", ErrPixInvalido, len(e2e))
	}
	for i := 0; i < len(e2e); i++ {
		c := e2e[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return fmt.Errorf("%w: endToEndId com caractere %q", ErrPixInvalido, c)
		}
	}
	return nil
}

// ErrPixInvalido indica um Pix recebido com dados fora do formato da spec.
var ErrPixInvalido = errors.New("pix recebido inválido")

// Devolucao é a devolução total ou parcial de um Pix recebido (RN-20, RN-21).
type Devolucao struct {
	ID         string
	EndToEndID string
	ContaID    string
	Valor      Money
	Status     StatusDevolucao
	Motivo     string
	CriadaEm   time.Time
}
