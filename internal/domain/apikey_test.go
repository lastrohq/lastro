package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var agora = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

func TestNovaAPIKeyGeraTokenQueConfere(t *testing.T) {
	k, token, err := NovaAPIKey("loja", []Escopo{EscopoCobWrite, EscopoCobRead, EscopoCobWrite}, "", agora)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "vtm_"+k.ID+"_") {
		t.Fatalf("token %q não começa com vtm_<id>_", token)
	}
	if strings.Contains(string(k.Hash), token[len("vtm_")+13:]) {
		t.Fatal("hash contém o segredo")
	}
	if len(k.Escopos) != 2 || k.Escopos[0] != EscopoCobRead {
		t.Fatalf("escopos não foram deduplicados e ordenados: %v", k.Escopos)
	}
	id, seg, err := SepararToken(token)
	if err != nil || id != k.ID {
		t.Fatalf("SepararToken = %q, %v", id, err)
	}
	if err := k.Confere(seg); err != nil {
		t.Fatal(err)
	}
	// Troca o último caractere por outro diferente (o segredo pode já terminar em "x").
	trocado := "x"
	if strings.HasSuffix(seg, "x") {
		trocado = "y"
	}
	if err := k.Confere(seg[:len(seg)-1] + trocado); !errors.Is(err, ErrNaoAutenticado) {
		t.Fatal("segredo errado conferiu")
	}
	revogada := agora
	k.RevogadaEm = &revogada
	if err := k.Confere(seg); !errors.Is(err, ErrNaoAutenticado) {
		t.Fatal("chave revogada conferiu")
	}
}

func TestNovaAPIKeyRecusa(t *testing.T) {
	casos := map[string]func() error{
		"sem nome":    func() error { _, _, err := NovaAPIKey("", []Escopo{EscopoCobRead}, "", agora); return err },
		"sem escopo":  func() error { _, _, err := NovaAPIKey("x", nil, "", agora); return err },
		"inventado":   func() error { _, _, err := NovaAPIKey("x", []Escopo{"tudo"}, "", agora); return err },
		"admin preso": func() error { _, _, err := NovaAPIKey("x", []Escopo{EscopoContaAdmin}, "loja", agora); return err },
	}
	for nome, f := range casos {
		if err := f(); !errors.Is(err, ErrAPIKeyInvalida) {
			t.Errorf("%s: err = %v", nome, err)
		}
	}
}

func TestSepararTokenRecusaFormato(t *testing.T) {
	for _, tok := range []string{"", "Bearer x", "vtm_", "vtm_abc_def", "sk_123456789012_12345678901234567890123456789012"} {
		if _, _, err := SepararToken(tok); !errors.Is(err, ErrNaoAutenticado) {
			t.Errorf("%q aceito", tok)
		}
	}
}

func TestPermite(t *testing.T) {
	k := APIKey{Escopos: []Escopo{EscopoCobRead}}
	if err := k.Permite(EscopoCobRead); err != nil {
		t.Fatal(err)
	}
	if err := k.Permite(EscopoCobWrite); !errors.Is(err, ErrSemEscopo) || !strings.Contains(err.Error(), "cob:write") {
		t.Fatalf("err = %v", err)
	}
}
