package efi_test

import (
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports/fake"
	"github.com/lastrohq/lastro/test/conformance"
)

// A suíte de conformidade roda contra o Efí falso, que segue os exemplos da
// documentação. As fixtures gravadas da homologação de verdade entram com a #21.
func TestConformidade(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Alvo {
		c := novo(t)
		relogio := fake.NewClock(agora)
		return conformance.Alvo{
			PSP:   c.psp,
			Conta: c.conta,
			Agora: relogio.Now,
			Pagar: func(_ *testing.T, txid domain.Txid) {
				c.efi.Pagar(string(txid), relogio.Now())
				relogio.Avancar(time.Second)
			},
		}
	})
}
