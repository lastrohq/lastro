package domain

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func contaValida() Conta {
	return Conta{ID: "loja-1", Nome: "Loja", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: Homologacao, Ativa: true}
}

func TestContaValidar(t *testing.T) {
	if err := contaValida().Validar(); err != nil {
		t.Fatal(err)
	}
	casos := map[string]func(*Conta){
		"id vazio":           func(c *Conta) { c.ID = "" },
		"id com maiúscula":   func(c *Conta) { c.ID = "Loja" },
		"id com espaço":      func(c *Conta) { c.ID = "lo ja" },
		"id começando com -": func(c *Conta) { c.ID = "-loja" },
		"id longo":           func(c *Conta) { c.ID = strings.Repeat("a", 64) },
		"sem nome":           func(c *Conta) { c.Nome = "" },
		"sem psp":            func(c *Conta) { c.PSP = "" },
		"sem chave":          func(c *Conta) { c.ChavePix = "" },
		"chave longa":        func(c *Conta) { c.ChavePix = strings.Repeat("a", 78) },
		"ambiente inventado": func(c *Conta) { c.Ambiente = "teste" },
	}
	for nome, mudar := range casos {
		c := contaValida()
		mudar(&c)
		if err := c.Validar(); !errors.Is(err, ErrContaInvalida) {
			t.Errorf("%s: err = %v", nome, err)
		}
	}
}

func TestContaInativaNaoCobra(t *testing.T) {
	c := contaValida()
	if err := c.PodeCobrar(); err != nil {
		t.Fatal(err)
	}
	c.Ativa = false
	if err := c.PodeCobrar(); !errors.Is(err, ErrContaInativa) {
		t.Fatalf("err = %v", err)
	}
}

func TestCredenciaisNuncaVazamEmTexto(t *testing.T) {
	cred := Credenciais{ClientID: "id-secreto", ClientSecret: "segredo-do-cliente", SenhaCertificado: "senha-do-cert"}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("teste", "cred", cred)
	saidas := []string{
		fmt.Sprint(cred), fmt.Sprintf("%v %+v %#v %s", cred, cred, cred, cred),
		fmt.Sprintf("%+v", struct{ C Credenciais }{cred}), buf.String(),
	}
	for _, s := range saidas {
		for _, segredo := range []string{"id-secreto", "segredo-do-cliente", "senha-do-cert"} {
			if strings.Contains(s, segredo) {
				t.Fatalf("vazou %q em %q", segredo, s)
			}
		}
	}
}
