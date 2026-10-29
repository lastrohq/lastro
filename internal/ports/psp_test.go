package ports

import (
	"errors"
	"strings"
	"testing"
)

func TestCapabilitiesSuporta(t *testing.T) {
	c := Capabilities{CapCob: true}
	if err := c.Suporta(CapCob); err != nil {
		t.Fatal(err)
	}
	err := c.Suporta(CapRec)
	if !errors.Is(err, ErrNaoSuportado) || !strings.Contains(err.Error(), "rec") {
		t.Fatalf("err = %v, quer ErrNaoSuportado citando rec", err)
	}
}
