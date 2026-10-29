package domain

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

// ErrDevolucaoInvalida indica uma devolução fora da RN-21.
var ErrDevolucaoInvalida = errors.New("devolução inválida")

// PrazoDevolucao é quanto tempo depois do pagamento o recebedor ainda pode
// devolver o Pix (RN-21).
const PrazoDevolucao = 90 * 24 * time.Hour

// NaturezaDevolucao diz o que está sendo devolvido, como na spec do BCB.
type NaturezaDevolucao string

// Naturezas da spec. RETIRADA é a devolução do Pix Saque ou Troco, que o
// Lastro não recebe; só ORIGINAL é aceita.
const (
	DevolucaoOriginal NaturezaDevolucao = "ORIGINAL"
	DevolucaoRetirada NaturezaDevolucao = "RETIRADA"
)

// Devolucao é a devolução total ou parcial de um Pix recebido (RN-20, RN-21).
// O par EndToEndID + ID identifica a devolução; o ID é escolhido pelo cliente.
type Devolucao struct {
	ID         string
	EndToEndID string
	ContaID    string
	Valor      Money
	Natureza   NaturezaDevolucao
	// Descricao é o texto do recebedor para o pagador, até 140 caracteres.
	Descricao string
	Status    StatusDevolucao
	// Motivo é o porquê de a devolução não ter sido realizada, vindo do banco.
	Motivo string
	// RtrID é o identificador da devolução no SPI, gerado pelo banco.
	RtrID       string
	CriadaEm    time.Time
	LiquidadaEm time.Time
	Revisao     int
}

// ValidarIDDevolucao confere o id da spec: 1 a 35 caracteres [a-zA-Z0-9].
func ValidarIDDevolucao(id string) error {
	if len(id) < 1 || len(id) > 35 {
		return fmt.Errorf("%w: o id deve ter de 1 a 35 caracteres, tem %d", ErrDevolucaoInvalida, len(id))
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return fmt.Errorf("%w: id com caractere %q", ErrDevolucaoInvalida, c)
		}
	}
	return nil
}

// ValidarSolicitacao confere a devolução contra o Pix e as devoluções que ele
// já tem, no instante agora, antes de chamar o banco (RN-21): id e valor no
// formato da spec, natureza ORIGINAL, dentro do prazo de 90 dias do pagamento e
// sem passar do que sobra do valor recebido. Devolução NAO_REALIZADO não
// consome o saldo.
func (d Devolucao) ValidarSolicitacao(pix PixRecebido, anteriores []Devolucao, agora time.Time) error {
	if err := ValidarIDDevolucao(d.ID); err != nil {
		return err
	}
	if d.EndToEndID != pix.EndToEndID || d.ContaID != pix.ContaID {
		return fmt.Errorf("%w: a devolução não é do Pix %s", ErrDevolucaoInvalida, pix.EndToEndID)
	}
	if d.Valor <= 0 {
		return fmt.Errorf("%w: valor deve ser positivo", ErrDevolucaoInvalida)
	}
	if d.Natureza != DevolucaoOriginal {
		return fmt.Errorf("%w: natureza %q não suportada; o Lastro só devolve Pix de cobrança (ORIGINAL)", ErrDevolucaoInvalida, d.Natureza)
	}
	if n := utf8.RuneCountInString(d.Descricao); n > 140 {
		return fmt.Errorf("%w: descrição com %d caracteres; o máximo é 140", ErrDevolucaoInvalida, n)
	}
	if agora.Sub(pix.Horario) > PrazoDevolucao {
		return fmt.Errorf("%w: o Pix foi pago em %s, há mais de 90 dias", ErrDevolucaoInvalida, pix.Horario.Format(time.RFC3339))
	}
	if saldo := SaldoDevolvivel(pix, anteriores); d.Valor > saldo {
		return fmt.Errorf("%w: devolução de %s passa do saldo de %s do Pix (recebido %s)", ErrDevolucaoInvalida, d.Valor, saldo, pix.Valor)
	}
	return nil
}

// SaldoDevolvivel é o que ainda pode ser devolvido do Pix: o valor recebido
// menos as devoluções que não terminaram NAO_REALIZADO.
func SaldoDevolvivel(pix PixRecebido, devolucoes []Devolucao) Money {
	saldo := pix.Valor
	for _, d := range devolucoes {
		if d.Status != DevolucaoNaoRealizado {
			saldo -= d.Valor
		}
	}
	return max(saldo, 0)
}

// Transitar muda o status respeitando a RN-20.
func (d *Devolucao) Transitar(para StatusDevolucao) error {
	if err := TransicoesDevolucao.Validar(d.Status, para); err != nil {
		return err
	}
	d.Status = para
	return nil
}
