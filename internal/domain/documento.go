package domain

import (
	"errors"
	"fmt"
)

// ErrDocumentoInvalido indica CPF ou CNPJ com formato ou dígito verificador errado.
var ErrDocumentoInvalido = errors.New("documento inválido")

// ValidarDocumento aceita CPF (11 dígitos) ou CNPJ (14 caracteres, alfanumérico
// nas 12 primeiras posições e dígitos verificadores numéricos), conferindo os
// dígitos verificadores. Sem pontuação.
func ValidarDocumento(doc string) error {
	switch len(doc) {
	case 11:
		return validarCPF(doc)
	case 14:
		return validarCNPJ(doc)
	}
	return fmt.Errorf("%w: %d caracteres (CPF tem 11, CNPJ tem 14)", ErrDocumentoInvalido, len(doc))
}

func validarCPF(cpf string) error {
	if !soDigitos(cpf) {
		return fmt.Errorf("%w: CPF só tem dígitos", ErrDocumentoInvalido)
	}
	if todosIguais(cpf) {
		return fmt.Errorf("%w: CPF com todos os dígitos iguais", ErrDocumentoInvalido)
	}
	for _, n := range []int{9, 10} {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(cpf[i]-'0') * (n + 1 - i)
		}
		dv := soma * 10 % 11 % 10
		if dv != int(cpf[n]-'0') {
			return fmt.Errorf("%w: dígito verificador do CPF", ErrDocumentoInvalido)
		}
	}
	return nil
}

// validarCNPJ segue a regra da Receita para o CNPJ alfanumérico: cada caractere
// vale seu código ASCII menos 48 ('0'=0 ... '9'=9, 'A'=17 ... 'Z'=42), com os
// mesmos pesos do módulo 11 do CNPJ numérico.
func validarCNPJ(cnpj string) error {
	for i := 0; i < 12; i++ {
		c := cnpj[i]
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') {
			return fmt.Errorf("%w: CNPJ aceita só dígitos e letras maiúsculas", ErrDocumentoInvalido)
		}
	}
	if !soDigitos(cnpj[12:]) {
		return fmt.Errorf("%w: os dígitos verificadores do CNPJ são numéricos", ErrDocumentoInvalido)
	}
	if todosIguais(cnpj) {
		return fmt.Errorf("%w: CNPJ com todos os caracteres iguais", ErrDocumentoInvalido)
	}
	pesos := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	for _, n := range []int{12, 13} {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(cnpj[i]-'0') * pesos[len(pesos)-n+i]
		}
		dv := 0
		if r := soma % 11; r >= 2 {
			dv = 11 - r
		}
		if dv != int(cnpj[n]-'0') {
			return fmt.Errorf("%w: dígito verificador do CNPJ", ErrDocumentoInvalido)
		}
	}
	return nil
}

func todosIguais(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}
