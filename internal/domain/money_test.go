package domain

import (
	"errors"
	"testing"
)

func TestParseMoney(t *testing.T) {
	casos := []struct {
		entrada string
		quer    Money
	}{
		{"0.01", 1},
		{"1.00", 100},
		{"1.50", 150},
		{"10.50", 1050},
		{"0.00", 0},
		{"01.00", 100},
		{"9999999999.99", MaxMoney},
	}
	for _, c := range casos {
		got, err := ParseMoney(c.entrada)
		if err != nil {
			t.Errorf("ParseMoney(%q) erro inesperado: %v", c.entrada, err)
			continue
		}
		if got != c.quer {
			t.Errorf("ParseMoney(%q) = %d, quer %d", c.entrada, got, c.quer)
		}
	}
}

func TestParseMoneyRecusaFormatoAmbiguo(t *testing.T) {
	for _, entrada := range []string{
		"", "1", "1.5", ".50", "10.", "10.505", "-1.00", "+1.00", "10,50", "1.000,00", "1,000.00",
		" 10.00", "10.00 ", "1e3", "abc", "10.5a", "10000000000.00", "99999999999999999999",
	} {
		if _, err := ParseMoney(entrada); !errors.Is(err, ErrValorInvalido) {
			t.Errorf("ParseMoney(%q) err = %v, quer ErrValorInvalido", entrada, err)
		}
	}
}

func TestMoneyString(t *testing.T) {
	casos := map[Money]string{0: "0.00", 1: "0.01", 150: "1.50", 1050: "10.50", MaxMoney: "9999999999.99", -150: "-1.50"}
	for m, quer := range casos {
		if got := m.String(); got != quer {
			t.Errorf("Money(%d).String() = %q, quer %q", int64(m), got, quer)
		}
	}
}

func TestMoneyValid(t *testing.T) {
	casos := map[Money]bool{0: false, -1: false, 1: true, MaxMoney: true, MaxMoney + 1: false}
	for m, quer := range casos {
		if got := m.Valid(); got != quer {
			t.Errorf("Money(%d).Valid() = %v, quer %v", int64(m), got, quer)
		}
	}
}

func FuzzMoneyRoundTrip(f *testing.F) {
	for _, s := range []int64{0, 1, 99, 100, 1050, int64(MaxMoney)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, centavos int64) {
		m := Money(centavos)
		if m < 0 || m > MaxMoney {
			t.Skip()
		}
		got, err := ParseMoney(m.String())
		if err != nil {
			t.Fatalf("ParseMoney(%q): %v", m.String(), err)
		}
		if got != m {
			t.Fatalf("round-trip %d -> %q -> %d", m, m.String(), got)
		}
	})
}

func FuzzParseMoneyNaoEntraEmPanico(f *testing.F) {
	for _, s := range []string{"10.50", "", "-1", "1e9", "0.001"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		m, err := ParseMoney(s)
		if err == nil && (m < 0 || m > MaxMoney) {
			t.Fatalf("ParseMoney(%q) = %d fora do intervalo sem erro", s, m)
		}
	})
}
