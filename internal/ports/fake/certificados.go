package fake

import "github.com/lastrohq/lastro/internal/domain"

// Certificados é o ports.Certificados que devolve sempre o mesmo resultado.
type Certificados struct {
	Certificado domain.Certificado
	Err         error
}

// Ler devolve Certificado e Err, sem olhar o arquivo.
func (c Certificados) Ler(_ []byte, _ string) (domain.Certificado, error) {
	return c.Certificado, c.Err
}
