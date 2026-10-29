package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/certificado"
)

// leitorCertificados é o ports.Certificados sobre platform/certificado, que não
// conhece o domínio (regra de camadas da ARQUITETURA.md).
type leitorCertificados struct{}

func (leitorCertificados) Ler(arquivo []byte, senha string) (domain.Certificado, error) {
	info, err := certificado.Ler(arquivo, senha)
	if errors.Is(err, certificado.ErrInvalido) {
		motivo := strings.TrimPrefix(err.Error(), certificado.ErrInvalido.Error()+": ")
		return domain.Certificado{}, fmt.Errorf("%w: %s", domain.ErrCertificadoInvalido, motivo)
	}
	if err != nil {
		return domain.Certificado{}, err
	}
	return domain.Certificado{Sujeito: info.Sujeito, ImpressaoDigital: info.ImpressaoDigital, Validade: info.Validade}, nil
}
