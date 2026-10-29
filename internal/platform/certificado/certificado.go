// Package certificado lê o certificado do mTLS de uma conta, em .p12 (com senha)
// ou PEM (certificado e chave no mesmo arquivo), e confere que a chave privada
// corresponde ao certificado. Nenhuma mensagem de erro traz o conteúdo do arquivo.
package certificado

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// ErrInvalido indica arquivo ilegível, senha errada ou chave que não corresponde
// ao certificado.
var ErrInvalido = errors.New("certificado inválido")

// Info são os dados públicos do certificado.
type Info struct {
	// Sujeito é o subject, como "CN=...,O=...".
	Sujeito string
	// ImpressaoDigital é o SHA-256 do certificado (DER), em hexadecimal minúsculo.
	ImpressaoDigital string
	// Validade é o NotAfter, em UTC.
	Validade time.Time
}

// Ler interpreta o arquivo como PEM, se tiver um bloco "-----BEGIN", ou como
// .p12. A senha só vale para o .p12.
func Ler(arquivo []byte, senha string) (Info, error) {
	par, err := TLS(arquivo, senha)
	if err != nil {
		return Info{}, err
	}
	return info(par.Leaf), nil
}

// TLS faz a mesma leitura de Ler e devolve o par pronto para o mTLS, com a
// cadeia do .p12 (se houver) e Leaf preenchido.
func TLS(arquivo []byte, senha string) (tls.Certificate, error) {
	if bytes.Contains(arquivo, []byte("-----BEGIN")) {
		return lerPEM(arquivo)
	}
	return lerP12(arquivo, senha)
}

func lerP12(arquivo []byte, senha string) (tls.Certificate, error) {
	chave, cert, cadeia, err := pkcs12.DecodeChain(arquivo, senha)
	switch {
	case errors.Is(err, pkcs12.ErrIncorrectPassword):
		return tls.Certificate{}, fmt.Errorf("%w: senha do .p12 errada", ErrInvalido)
	case err != nil:
		return tls.Certificate{}, fmt.Errorf("%w: o arquivo não é um .p12 nem um PEM legível", ErrInvalido)
	}
	assinador, ok := chave.(crypto.Signer)
	if !ok {
		return tls.Certificate{}, fmt.Errorf("%w: tipo de chave privada não suportado", ErrInvalido)
	}
	publica, ok := cert.PublicKey.(interface{ Equal(crypto.PublicKey) bool })
	if !ok || !publica.Equal(assinador.Public()) {
		return tls.Certificate{}, fmt.Errorf("%w: a chave privada do .p12 não corresponde ao certificado", ErrInvalido)
	}
	par := tls.Certificate{Certificate: [][]byte{cert.Raw}, PrivateKey: chave, Leaf: cert}
	for _, c := range cadeia {
		par.Certificate = append(par.Certificate, c.Raw)
	}
	return par, nil
}

func lerPEM(arquivo []byte) (tls.Certificate, error) {
	var temCert, temChave bool
	for resto := arquivo; ; {
		var bloco *pem.Block
		if bloco, resto = pem.Decode(resto); bloco == nil {
			break
		}
		switch {
		case bloco.Type == "CERTIFICATE":
			temCert = true
		case bloco.Type == "ENCRYPTED PRIVATE KEY" || bloco.Headers["Proc-Type"] == "4,ENCRYPTED":
			return tls.Certificate{}, fmt.Errorf("%w: chave PEM cifrada não é suportada (exporte a chave sem senha ou use o .p12)", ErrInvalido)
		case strings.HasSuffix(bloco.Type, "PRIVATE KEY"):
			temChave = true
		}
	}
	switch {
	case !temCert:
		return tls.Certificate{}, fmt.Errorf("%w: o PEM não tem bloco CERTIFICATE", ErrInvalido)
	case !temChave:
		return tls.Certificate{}, fmt.Errorf("%w: o PEM não tem a chave privada (junte certificado e chave no mesmo arquivo)", ErrInvalido)
	}
	par, err := tls.X509KeyPair(arquivo, arquivo)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("%w: chave privada inválida ou que não corresponde ao certificado", ErrInvalido)
	}
	if par.Leaf == nil {
		if par.Leaf, err = x509.ParseCertificate(par.Certificate[0]); err != nil {
			return tls.Certificate{}, fmt.Errorf("%w: certificado ilegível", ErrInvalido)
		}
	}
	return par, nil
}

func info(cert *x509.Certificate) Info {
	soma := sha256.Sum256(cert.Raw)
	return Info{
		Sujeito:          cert.Subject.String(),
		ImpressaoDigital: hex.EncodeToString(soma[:]),
		Validade:         cert.NotAfter.UTC(),
	}
}
