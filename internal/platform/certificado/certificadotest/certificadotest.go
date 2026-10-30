// Package certificadotest gera certificados autoassinados para testes: nada
// aqui serve para produção.
package certificadotest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"software.sslmate.com/src/go-pkcs12"
)

// Senha é a senha dos .p12 gerados.
const Senha = "senha-de-teste" //nolint:gosec // G101: senha de certificado de teste

// Par é um certificado gerado, com a chave, nos dois formatos aceitos.
type Par struct {
	Certificado *x509.Certificate
	// P12 usa o formato moderno (AES-256) e a Senha.
	P12 []byte
	// P12Legado usa o formato antigo (RC2/3DES), ainda comum em banco.
	P12Legado []byte
	// PEM tem o certificado e a chave no mesmo arquivo, sem senha.
	PEM []byte
	// ChavePEM é só a chave privada, para montar arquivos inválidos.
	ChavePEM []byte
	// CertificadoPEM é só o certificado.
	CertificadoPEM []byte
}

// Gerar cria um certificado com o CN dado, válido até validade. Se ele for usado
// num handshake TLS, calcule validade a partir de time.Now: o TLS confere o
// certificado no relógio real, não no relógio falso do teste.
func Gerar(t testing.TB, cn string, validade time.Time) Par {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	modelo := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn, Organization: []string{"Vintém Teste"}},
		NotBefore:    validade.Add(-365 * 24 * time.Hour),
		NotAfter:     validade,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	chaveDER, err := x509.MarshalPKCS8PrivateKey(chave)
	if err != nil {
		t.Fatal(err)
	}
	p12, err := pkcs12.Modern.Encode(chave, cert, nil, Senha)
	if err != nil {
		t.Fatal(err)
	}
	legado, err := pkcs12.LegacyDES.Encode(chave, cert, nil, Senha)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	chavePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: chaveDER})
	return Par{
		Certificado:    cert,
		P12:            p12,
		P12Legado:      legado,
		PEM:            append(append([]byte{}, certPEM...), chavePEM...),
		ChavePEM:       chavePEM,
		CertificadoPEM: certPEM,
	}
}
