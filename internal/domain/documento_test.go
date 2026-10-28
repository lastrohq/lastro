package domain

import (
	"errors"
	"testing"
)

func TestValidarDocumento(t *testing.T) {
	validos := []string{
		"12345678909",    // CPF clássico de teste
		"52998224725",    // CPF
		"11222333000181", // CNPJ numérico
		"12ABC34501DE35", // CNPJ alfanumérico do exemplo da Receita
	}
	for _, d := range validos {
		if err := ValidarDocumento(d); err != nil {
			t.Errorf("%s: %v", d, err)
		}
	}
	invalidos := []string{
		"", "123", "12345678900", "11111111111", "1234567890a",
		"11222333000180", "11111111111111", "12abc34501DE35", "12ABC34501DEAB", "123.456.789-09",
	}
	for _, d := range invalidos {
		if err := ValidarDocumento(d); !errors.Is(err, ErrDocumentoInvalido) {
			t.Errorf("%q aceito (err = %v)", d, err)
		}
	}
}

func FuzzValidarDocumentoNaoEntraEmPanico(f *testing.F) {
	f.Add("12ABC34501DE35")
	f.Fuzz(func(_ *testing.T, s string) { _ = ValidarDocumento(s) })
}
