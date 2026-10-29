// Package clock tem o relógio de verdade; nos testes, use internal/ports/fake.
package clock

import "time"

// Sistema é o ports.Clock que lê o relógio do sistema, sempre em UTC.
type Sistema struct{}

// Now devolve o instante atual em UTC.
func (Sistema) Now() time.Time { return time.Now().UTC() }
