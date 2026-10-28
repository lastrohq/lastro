package domain

import (
	"errors"
	"fmt"
	"strconv"
)

// Money é um valor em centavos de real. Dinheiro nunca é float (RN-02).
type Money int64

// MaxMoney é o maior valor que a spec do BCB representa: 10 dígitos inteiros
// e 2 decimais, ou R$ 9.999.999.999,99.
const MaxMoney Money = 999_999_999_999

// ErrValorInvalido indica uma string de valor fora do formato da spec do BCB.
var ErrValorInvalido = errors.New("valor inválido")

// ParseMoney converte a string decimal da spec do BCB em centavos.
// O formato aceito é exatamente o da spec, ^\d{1,10}\.\d{2}$: sinal, vírgula,
// separador de milhar, espaço e casas decimais a mais ou a menos são recusados.
func ParseMoney(s string) (Money, error) {
	n := len(s)
	if n < 4 || n > 13 || s[n-3] != '.' || !soDigitos(s[:n-3]) || !soDigitos(s[n-2:]) {
		return 0, fmt.Errorf("%w: %q (formato esperado: 10.50)", ErrValorInvalido, s)
	}
	reais, _ := strconv.ParseInt(s[:n-3], 10, 64)
	centavos, _ := strconv.ParseInt(s[n-2:], 10, 64)
	return Money(reais*100 + centavos), nil
}

// String devolve o valor no formato da spec do BCB, sempre com duas casas ("10.50").
func (m Money) String() string {
	if m < 0 {
		return "-" + (-m).String()
	}
	return fmt.Sprintf("%d.%02d", int64(m)/100, int64(m)%100)
}

// Valid informa se o valor pode ser cobrado: positivo e dentro do máximo da spec.
func (m Money) Valid() bool { return m > 0 && m <= MaxMoney }

func soDigitos(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
