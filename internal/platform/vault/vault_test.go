package vault

import (
	"bytes"
	"errors"
	"testing"

	"github.com/vintemhq/vintem/internal/ports"
)

var _ ports.Vault = (*AESGCM)(nil)

func novo(t *testing.T) *AESGCM {
	t.Helper()
	v, err := New(NovaChave())
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCifrarDecifrar(t *testing.T) {
	v := novo(t)
	segredo := []byte("client-secret-muito-secreto")
	c, err := v.Cifrar(t.Context(), segredo)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(c, segredo) {
		t.Fatal("texto cifrado contém o segredo em claro")
	}
	got, err := v.Decifrar(t.Context(), c)
	if err != nil || !bytes.Equal(got, segredo) {
		t.Fatalf("Decifrar = %q, %v", got, err)
	}
}

func TestMesmoSegredoCifraDiferenteACadaVez(t *testing.T) {
	v := novo(t)
	a, _ := v.Cifrar(t.Context(), []byte("x"))
	b, _ := v.Cifrar(t.Context(), []byte("x"))
	if bytes.Equal(a, b) {
		t.Fatal("nonce repetido")
	}
}

func TestDecifrarRecusaAdulteradoOuOutraChave(t *testing.T) {
	v := novo(t)
	c, _ := v.Cifrar(t.Context(), []byte("segredo"))
	for i := range c {
		adulterado := bytes.Clone(c)
		adulterado[i] ^= 0x01
		if _, err := v.Decifrar(t.Context(), adulterado); !errors.Is(err, ErrDecifrar) {
			t.Fatalf("byte %d adulterado foi aceito", i)
		}
	}
	if _, err := novo(t).Decifrar(t.Context(), c); !errors.Is(err, ErrDecifrar) {
		t.Fatal("outra chave decifrou")
	}
	if _, err := v.Decifrar(t.Context(), nil); !errors.Is(err, ErrDecifrar) {
		t.Fatal("vazio foi aceito")
	}
}

func TestChaveInvalida(t *testing.T) {
	for _, k := range []string{"", "nao-e-base64!!", "c2hvcnQ="} {
		if _, err := New(k); !errors.Is(err, ErrChaveInvalida) {
			t.Errorf("New(%q) err = %v", k, err)
		}
	}
}
