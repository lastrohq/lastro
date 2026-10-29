package brcode

import (
	"errors"
	"strings"
	"testing"
)

func TestEncodeReproduzOExemploDoManual(t *testing.T) {
	got, err := Encode(Pix{Chave: "123e4567-e12b-12d1-a456-426655440000", Nome: "Fulano de Tal", Cidade: "BRASILIA"})
	if err != nil {
		t.Fatal(err)
	}
	if got != exemploManualBCB {
		t.Fatalf("\n got %s\nquer %s", got, exemploManualBCB)
	}
}

func TestDecodeDoExemploDoManual(t *testing.T) {
	p, err := Decode(exemploManualBCB)
	if err != nil {
		t.Fatal(err)
	}
	quer := Pix{Chave: "123e4567-e12b-12d1-a456-426655440000", Nome: "Fulano de Tal", Cidade: "BRASILIA"}
	if p != quer {
		t.Fatalf("got %+v, quer %+v", p, quer)
	}
}

func TestRoundTrip(t *testing.T) {
	casos := []Pix{
		{Chave: "fulano@example.com", Nome: "Fulano", Cidade: "SAO PAULO", Valor: 1050, Txid: "PEDIDO123", Descricao: "Pedido 123"},
		{Chave: "+5511999999999", Nome: "Loja", Cidade: "RECIFE", Valor: 1},
		{URL: "pix.example.com/qr/v2/9d36b84fc70b478fb95c12729b90ca25", Nome: "Loja", Cidade: "CURITIBA", Valor: 999_999_999_999, UnicoUso: true},
		{URL: "pix.example.com/qr/v2/cobv/abc", Nome: "Loja", Cidade: "NATAL"},
	}
	for _, p := range casos {
		code, err := Encode(p)
		if err != nil {
			t.Fatalf("Encode(%+v): %v", p, err)
		}
		got, err := Decode(code)
		if err != nil {
			t.Fatalf("Decode(%s): %v", code, err)
		}
		if got != p {
			t.Fatalf("round-trip\n got %+v\nquer %+v", got, p)
		}
	}
}

func TestEncodeRemoveAcentosDeNomeECidade(t *testing.T) {
	code, err := Encode(Pix{Chave: "x@example.com", Nome: "João & Cia", Cidade: "São Gonçalo"})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := Decode(code)
	if p.Nome != "Joao & Cia" || p.Cidade != "Sao Goncalo" {
		t.Fatalf("nome=%q cidade=%q", p.Nome, p.Cidade)
	}
}

func TestEncodeRecusa(t *testing.T) {
	base := Pix{Chave: "x@example.com", Nome: "Loja", Cidade: "RIO"}
	casos := map[string]func(*Pix){
		"sem chave nem URL":     func(p *Pix) { p.Chave = "" },
		"chave e URL":           func(p *Pix) { p.URL = "pix.example.com/x" },
		"URL com esquema":       func(p *Pix) { p.Chave, p.URL = "", "https://pix.example.com/x" },
		"descrição no dinâmico": func(p *Pix) { p.Chave, p.URL, p.Descricao = "", "pix.example.com/x", "oi" },
		"txid no dinâmico":      func(p *Pix) { p.Chave, p.URL, p.Txid = "", "pix.example.com/x", "ABC" },
		"nome vazio":            func(p *Pix) { p.Nome = " " },
		"nome longo":            func(p *Pix) { p.Nome = strings.Repeat("a", 26) },
		"cidade longa":          func(p *Pix) { p.Cidade = strings.Repeat("a", 16) },
		"nome não ASCII":        func(p *Pix) { p.Nome = "Loja 😀" },
		"valor negativo":        func(p *Pix) { p.Valor = -1 },
		"valor acima do máximo": func(p *Pix) { p.Valor = 1_000_000_000_000 },
		"txid longo":            func(p *Pix) { p.Txid = strings.Repeat("a", 26) },
		"txid com símbolo":      func(p *Pix) { p.Txid = "pedido-1" },
		"chave longa":           func(p *Pix) { p.Chave = strings.Repeat("a", 78) },
		"campo 26 estourado":    func(p *Pix) { p.Chave, p.Descricao = strings.Repeat("a", 70), strings.Repeat("b", 10) },
	}
	for nome, mudar := range casos {
		p := base
		mudar(&p)
		if _, err := Encode(p); !errors.Is(err, ErrInvalido) {
			t.Errorf("%s: err = %v, quer ErrInvalido", nome, err)
		}
	}
}

func TestDecodeRecusa(t *testing.T) {
	semCRC := exemploManualBCB[:len(exemploManualBCB)-8]
	casos := map[string]string{
		"vazio":          "",
		"CRC errado":     semCRC + "63040000",
		"sem campo 63":   semCRC,
		"CRC não hex":    semCRC + "6304ZZZZ",
		"truncado":       comCRC("000201260"),
		"sem nome":       comCRC(strings.Replace(semCRC, "5913Fulano de Tal", "", 1)),
		"moeda errada":   comCRC(strings.Replace(semCRC, "5303986", "5303840", 1)),
		"outro arranjo":  comCRC(strings.Replace(semCRC, "br.gov.bcb.pix", "br.gov.bcb.xyz", 1)),
		"campo repetido": comCRC(semCRC + "5802BR"),
		"valor inválido": comCRC(strings.Replace(semCRC, "5802BR", "54041,005802BR", 1)),
	}
	for nome, code := range casos {
		if _, err := Decode(code); !errors.Is(err, ErrInvalido) {
			t.Errorf("%s: err = %v, quer ErrInvalido", nome, err)
		}
	}
}

func TestDecodeValorSemCasasDecimais(t *testing.T) {
	semCRC := exemploManualBCB[:len(exemploManualBCB)-8]
	p, err := Decode(comCRC(strings.Replace(semCRC, "5802BR", "540210"+"5802BR", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Valor != 1000 {
		t.Fatalf("valor = %d, quer 1000", p.Valor)
	}
}

func FuzzDecodeNaoEntraEmPanico(f *testing.F) {
	f.Add(exemploManualBCB)
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		p, err := Decode(s)
		if err != nil {
			return
		}
		if p.Nome == "" || p.Cidade == "" || (p.Chave == "") == (p.URL == "") {
			t.Fatalf("Decode aceitou BR Code incompleto: %+v", p)
		}
	})
}
