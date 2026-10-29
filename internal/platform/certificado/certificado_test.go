package certificado_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/platform/certificado"
	"github.com/lastrohq/lastro/internal/platform/certificado/certificadotest"
)

var validade = time.Date(2027, 6, 30, 23, 59, 59, 0, time.UTC)

func TestLerFormatosAceitos(t *testing.T) {
	par := certificadotest.Gerar(t, "loja.example.com", validade)
	soma := sha256.Sum256(par.Certificado.Raw)
	quer := certificado.Info{
		Sujeito:          "CN=loja.example.com,O=Lastro Teste",
		ImpressaoDigital: hex.EncodeToString(soma[:]),
		Validade:         validade,
	}
	for nome, c := range map[string]struct {
		arquivo []byte
		senha   string
	}{
		"p12 moderno (AES)":        {par.P12, certificadotest.Senha},
		"p12 legado (3DES)":        {par.P12Legado, certificadotest.Senha},
		"PEM com cert e chave":     {par.PEM, ""},
		"PEM com a chave primeiro": {append(append([]byte{}, par.ChavePEM...), par.CertificadoPEM...), ""},
		"PEM ignora a senha":       {par.PEM, "qualquer"},
	} {
		t.Run(nome, func(t *testing.T) {
			got, err := certificado.Ler(c.arquivo, c.senha)
			if err != nil {
				t.Fatal(err)
			}
			if got != quer {
				t.Fatalf("Ler = %+v\nquer   %+v", got, quer)
			}
		})
	}
}

func TestLerRecusa(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", validade)
	outro := certificadotest.Gerar(t, "outra", validade)
	cifrada := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte{1, 2, 3}})

	for nome, c := range map[string]struct {
		arquivo []byte
		senha   string
		motivo  string
	}{
		"senha errada":           {par.P12, "errada", "senha do .p12 errada"},
		"lixo":                   {[]byte("não sou um certificado"), "", "não é um .p12 nem um PEM"},
		"vazio":                  {nil, "", "não é um .p12 nem um PEM"},
		"PEM só com certificado": {par.CertificadoPEM, "", "não tem a chave privada"},
		"PEM só com chave":       {par.ChavePEM, "", "não tem bloco CERTIFICATE"},
		"chave de outro":         {append(append([]byte{}, par.CertificadoPEM...), outro.ChavePEM...), "", "não corresponde ao certificado"},
		"chave PEM cifrada":      {append(append([]byte{}, par.CertificadoPEM...), cifrada...), "", "chave PEM cifrada"},
	} {
		t.Run(nome, func(t *testing.T) {
			_, err := certificado.Ler(c.arquivo, c.senha)
			if !errors.Is(err, certificado.ErrInvalido) {
				t.Fatalf("erro = %v; quer ErrInvalido", err)
			}
			if !strings.Contains(err.Error(), c.motivo) {
				t.Fatalf("erro %q não explica o motivo %q", err, c.motivo)
			}
		})
	}
}

func TestErroNaoMostraOConteudo(t *testing.T) {
	par := certificadotest.Gerar(t, "loja", validade)
	outro := certificadotest.Gerar(t, "outra", validade)
	arquivo := append(append([]byte{}, par.CertificadoPEM...), outro.ChavePEM...)
	_, err := certificado.Ler(arquivo, "")
	for _, trecho := range strings.Split(string(outro.ChavePEM), "\n")[1:3] {
		if strings.Contains(err.Error(), trecho) {
			t.Fatalf("a mensagem de erro mostra a chave: %v", err)
		}
	}
}
