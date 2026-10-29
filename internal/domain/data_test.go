package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDataDeUsaOFusoDeBrasilia(t *testing.T) {
	// 01:30 UTC de 30/11 ainda é 29/11 em Brasília (UTC-3).
	if got := DataDe(time.Date(2026, 11, 30, 1, 30, 0, 0, time.UTC)).String(); got != "2026-11-29" {
		t.Fatalf("DataDe = %s, quer 2026-11-29", got)
	}
}

func TestParseDataESomarDias(t *testing.T) {
	d, err := ParseData("2026-12-30")
	if err != nil {
		t.Fatal(err)
	}
	if got := d.SomarDias(3).String(); got != "2027-01-02" {
		t.Fatalf("SomarDias = %s", got)
	}
	if !d.Antes(d.SomarDias(1)) || d.Antes(d) || !d.SomarDias(1).Depois(d) {
		t.Fatal("comparações erradas")
	}
	for _, s := range []string{"", "30/12/2026", "2026-02-30", "2026-1-5"} {
		if _, err := ParseData(s); !errors.Is(err, ErrDataInvalida) {
			t.Errorf("%q aceito", s)
		}
	}
}

func TestCobvExpiraDepoisDaValidade(t *testing.T) {
	c := CobrancaVencimento{Status: CobrancaAtiva, Vencimento: NovaData(2026, 11, 30), ValidadeAposVencimento: 2}
	ultimoDia := time.Date(2026, 12, 2, 23, 0, 0, 0, Brasilia)
	if c.Expirada(ultimoDia) {
		t.Fatal("expirou no último dia")
	}
	if !c.Expirada(ultimoDia.Add(2 * time.Hour)) {
		t.Fatal("não expirou no dia seguinte")
	}
}
