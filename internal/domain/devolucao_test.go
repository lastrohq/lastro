package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidarSolicitacaoDeDevolucao(t *testing.T) {
	pago := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	pix := PixRecebido{EndToEndID: "E18236120202610011200abcdefghijk", ContaID: "c1", Valor: 10000, Horario: pago}
	valida := func() Devolucao {
		return Devolucao{ID: "dev1", EndToEndID: pix.EndToEndID, ContaID: "c1", Valor: 4000, Natureza: DevolucaoOriginal, Descricao: "Produto devolvido"}
	}
	anteriores := []Devolucao{
		{ID: "a", Valor: 3000, Status: DevolucaoDevolvido},
		{ID: "b", Valor: 2000, Status: DevolucaoEmProcessamento},
		{ID: "c", Valor: 9000, Status: DevolucaoNaoRealizado}, // não consome o saldo
	}
	agora := pago.Add(24 * time.Hour)

	if err := valida().ValidarSolicitacao(pix, anteriores, agora); err != nil {
		t.Fatalf("válida recusada: %v", err)
	}
	total := valida()
	total.Valor = 5000
	if err := total.ValidarSolicitacao(pix, anteriores, agora); err != nil {
		t.Fatalf("devolver exatamente o saldo foi recusado: %v", err)
	}
	if err := valida().ValidarSolicitacao(pix, anteriores, pago.Add(PrazoDevolucao)); err != nil {
		t.Fatalf("no último instante do prazo foi recusada: %v", err)
	}

	casos := map[string]struct {
		mudar  func(*Devolucao)
		agora  time.Time
		contem string
	}{
		"id vazio":          {mudar: func(d *Devolucao) { d.ID = "" }, contem: "1 a 35"},
		"id longo":          {mudar: func(d *Devolucao) { d.ID = strings.Repeat("a", 36) }, contem: "1 a 35"},
		"id com hífen":      {mudar: func(d *Devolucao) { d.ID = "dev-1" }, contem: "caractere"},
		"outro pix":         {mudar: func(d *Devolucao) { d.EndToEndID = "E18236120202610011200zzzzzzzzzzz" }, contem: "não é do Pix"},
		"outra conta":       {mudar: func(d *Devolucao) { d.ContaID = "c2" }, contem: "não é do Pix"},
		"valor zero":        {mudar: func(d *Devolucao) { d.Valor = 0 }, contem: "positivo"},
		"passa do saldo":    {mudar: func(d *Devolucao) { d.Valor = 5001 }, contem: "saldo de 50.00"},
		"natureza retirada": {mudar: func(d *Devolucao) { d.Natureza = DevolucaoRetirada }, contem: "natureza"},
		"natureza vazia":    {mudar: func(d *Devolucao) { d.Natureza = "" }, contem: "natureza"},
		"descrição longa":   {mudar: func(d *Devolucao) { d.Descricao = strings.Repeat("é", 141) }, contem: "141 caracteres"},
		"depois de 90 dias": {mudar: func(*Devolucao) {}, agora: pago.Add(PrazoDevolucao + time.Second), contem: "90 dias"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			d := valida()
			c.mudar(&d)
			quando := agora
			if !c.agora.IsZero() {
				quando = c.agora
			}
			err := d.ValidarSolicitacao(pix, anteriores, quando)
			if !errors.Is(err, ErrDevolucaoInvalida) || !strings.Contains(err.Error(), c.contem) {
				t.Fatalf("err = %v, quer ErrDevolucaoInvalida com %q", err, c.contem)
			}
		})
	}
}

func TestSaldoDevolvivelNuncaNegativo(t *testing.T) {
	pix := PixRecebido{Valor: 1000}
	if s := SaldoDevolvivel(pix, []Devolucao{{Valor: 1500, Status: DevolucaoDevolvido}}); s != 0 {
		t.Fatalf("saldo = %s", s)
	}
}

func TestDevolucaoTransitar(t *testing.T) {
	d := Devolucao{Status: DevolucaoEmProcessamento}
	if err := d.Transitar(DevolucaoDevolvido); err != nil || d.Status != DevolucaoDevolvido {
		t.Fatalf("EM_PROCESSAMENTO → DEVOLVIDO: %v", err)
	}
	if err := d.Transitar(DevolucaoNaoRealizado); !errors.Is(err, ErrTransicaoInvalida) {
		t.Fatalf("DEVOLVIDO → NAO_REALIZADO: %v", err)
	}
}
