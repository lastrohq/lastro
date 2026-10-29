package domain

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"
)

// Ambiente diz se a conta fala com a homologação ou com a produção do banco.
type Ambiente string

// Ambientes possíveis de uma conta.
const (
	Homologacao Ambiente = "homologacao"
	Producao    Ambiente = "producao"
)

var (
	// ErrContaInvalida indica dados de conta fora das regras.
	ErrContaInvalida = errors.New("conta inválida")
	// ErrContaInativa indica uma conta desativada, que não aceita novas cobranças (RN-03).
	ErrContaInativa = errors.New("conta inativa")
	// ErrCertificadoInvalido indica certificado ilegível, senha errada ou chave que
	// não corresponde ao certificado (RN-62). É uma ErrContaInvalida.
	ErrCertificadoInvalido = fmt.Errorf("%w: certificado", ErrContaInvalida)
	// ErrCertificadoVencido indica certificado com validade no passado (RN-62).
	// É uma ErrContaInvalida.
	ErrCertificadoVencido = fmt.Errorf("%w: certificado vencido", ErrContaInvalida)
)

var idConta = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Conta é uma conta recebedora: um PSP, uma chave Pix e um ambiente (RN-03).
// As credenciais ficam cifradas à parte (RN-61), nunca nesta struct.
type Conta struct {
	ID       string
	Nome     string
	PSP      string
	ChavePix string
	Ambiente Ambiente
	Ativa    bool
	// Certificado tem os dados públicos do certificado do mTLS; nil se a conta
	// não tiver certificado (o sandbox, por exemplo).
	Certificado *Certificado
}

// Certificado são os dados públicos do certificado do mTLS, guardados fora do
// blob cifrado para o alerta de vencimento e a regra de ambiente (RN-62). O
// certificado e a chave privada continuam só nas Credenciais.
type Certificado struct {
	// Sujeito é o subject do certificado, como "CN=...,O=...".
	Sujeito string
	// ImpressaoDigital é o SHA-256 do certificado (DER), em hexadecimal minúsculo.
	ImpressaoDigital string
	// Validade é o NotAfter, em UTC.
	Validade time.Time
}

// VencidoEm diz se o certificado já não vale no instante agora.
func (c Certificado) VencidoEm(agora time.Time) bool { return !agora.Before(c.Validade) }

// Validar confere os campos obrigatórios e o formato do ID.
func (c Conta) Validar() error {
	switch {
	case !idConta.MatchString(c.ID):
		return fmt.Errorf("%w: id %q deve ter até 63 caracteres em [a-z0-9-], começando por letra ou dígito", ErrContaInvalida, c.ID)
	case c.Nome == "":
		return fmt.Errorf("%w: nome é obrigatório", ErrContaInvalida)
	case c.PSP == "":
		return fmt.Errorf("%w: psp é obrigatório", ErrContaInvalida)
	case c.ChavePix == "" || len(c.ChavePix) > 77:
		return fmt.Errorf("%w: chave Pix deve ter de 1 a 77 caracteres", ErrContaInvalida)
	case c.Ambiente != Homologacao && c.Ambiente != Producao:
		return fmt.Errorf("%w: ambiente %q (use %s ou %s)", ErrContaInvalida, c.Ambiente, Homologacao, Producao)
	}
	return nil
}

// PodeCobrar devolve ErrContaInativa se a conta estiver desativada.
func (c Conta) PodeCobrar() error {
	if !c.Ativa {
		return fmt.Errorf("conta %s: %w", c.ID, ErrContaInativa)
	}
	return nil
}

// Credenciais são o que o PSP exige para autenticar: OAuth2 client
// credentials e o certificado do mTLS. Nunca aparecem em log nem em resposta:
// String, GoString e LogValue escondem o conteúdo (RN-61).
type Credenciais struct {
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	Certificado      []byte `json:"certificado,omitempty"`
	SenhaCertificado string `json:"senhaCertificado,omitempty"`
}

const credenciaisOcultas = "[credenciais ocultas]" //nolint:gosec // G101: é o marcador de ocultação, não uma credencial

// String esconde o conteúdo em %v e %s.
func (Credenciais) String() string { return credenciaisOcultas }

// GoString esconde o conteúdo em %#v.
func (Credenciais) GoString() string { return credenciaisOcultas }

// LogValue impede que o slog serialize os campos.
func (Credenciais) LogValue() slog.Value { return slog.StringValue(credenciaisOcultas) }
