package domain

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
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

func TestCertificadoVencidoEm(t *testing.T) {
	validade := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	c := Certificado{Validade: validade}
	if c.VencidoEm(validade.Add(-time.Second)) {
		t.Fatal("um segundo antes da validade, já vencido")
	}
	if !c.VencidoEm(validade) {
		t.Fatal("no instante da validade, ainda vale")
	}
}

func TestErrosDeCertificadoSaoContaInvalida(t *testing.T) {
	for _, err := range []error{ErrCertificadoInvalido, ErrCertificadoVencido} {
		if !errors.Is(err, ErrContaInvalida) {
			t.Fatalf("%v não é ErrContaInvalida", err)
		}
	}
}

func TestCertificadoPrecisaAvisarEm(t *testing.T) {
	validade := time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	c := Certificado{Validade: validade}
	casos := []struct {
		agora time.Time
		quer  bool
	}{
		{validade.Add(-AntecedenciaAvisoVencimento - time.Second), false}, // 30 dias e 1 s antes
		{validade.Add(-AntecedenciaAvisoVencimento), true},                // exatamente 30 dias antes
		{validade.Add(-time.Hour), true},
		{validade.Add(24 * time.Hour), true}, // já vencido e nunca avisado
	}
	for _, caso := range casos {
		if got := c.PrecisaAvisarEm(caso.agora); got != caso.quer {
			t.Errorf("PrecisaAvisarEm(%s) = %v; quer %v", caso.agora, got, caso.quer)
		}
	}
	c.VencimentoAvisado = true
	if c.PrecisaAvisarEm(validade.Add(-time.Hour)) {
		t.Error("avisa de novo um certificado já avisado")
	}
}
