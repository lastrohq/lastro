package domain

import (
	"errors"
	"testing"
	"time"
)

// agoraCobr é quinta, 29/10/2026, 15h em Brasília.
var agoraCobr = time.Date(2026, 10, 29, 18, 0, 0, 0, time.UTC)

func recAprovada() Recorrencia {
	r := recValida()
	r.ID, r.Status = "RR1234567820261029abcDEF12345", RecorrenciaAprovada
	r.DataInicial = NovaData(2026, 10, 5)
	return r
}

func cobrValida() CobrancaRecorrente {
	return CobrancaRecorrente{
		Txid: "cobr0000000000000000000000001", RecorrenciaID: "RR1234567820261029abcDEF12345", ContaID: "c1",
		Status: CobrAtiva, Valor: 9990, Vencimento: NovaData(2026, 11, 5), AjusteDiaUtil: true,
	}
}

func TestCobrValidaNaJanelaDeAntecedencia(t *testing.T) {
	for _, venc := range []Data{NovaData(2026, 10, 31), NovaData(2026, 11, 5), NovaData(2026, 11, 8)} {
		c := cobrValida()
		c.Vencimento = venc
		if err := c.ValidarCriacao(recAprovada(), agoraCobr); err != nil {
			t.Errorf("vencimento %s: %v", venc, err)
		}
	}
	variavel := recAprovada()
	variavel.ValorFixo, variavel.PisoValorMaximo = 0, 5000
	c := cobrValida()
	c.Valor = 12345
	if err := c.ValidarCriacao(variavel, agoraCobr); err != nil {
		t.Errorf("valor variável: %v", err)
	}
}

func TestCobrInvalida(t *testing.T) {
	casos := map[string]func(*CobrancaRecorrente, *Recorrencia){
		"outra recorrência":       func(c *CobrancaRecorrente, _ *Recorrencia) { c.RecorrenciaID = "RN0000000020261029AAAAAAAAAAA" },
		"recorrência só criada":   func(_ *CobrancaRecorrente, r *Recorrencia) { r.Status = RecorrenciaCriada },
		"recorrência cancelada":   func(_ *CobrancaRecorrente, r *Recorrencia) { r.Status = RecorrenciaCancelada },
		"valor zero":              func(c *CobrancaRecorrente, _ *Recorrencia) { c.Valor = 0 },
		"valor diferente do fixo": func(c *CobrancaRecorrente, _ *Recorrencia) { c.Valor = 9991 },
		"infoAdicional longa":     func(c *CobrancaRecorrente, _ *Recorrencia) { c.InfoAdicional = string(make([]byte, 141)) },
		"sem vencimento":          func(c *CobrancaRecorrente, _ *Recorrencia) { c.Vencimento = Data{} },
		"antes do início":         func(_ *CobrancaRecorrente, r *Recorrencia) { r.DataInicial = NovaData(2026, 11, 6) },
		"depois do fim":           func(_ *CobrancaRecorrente, r *Recorrencia) { r.DataFinal = NovaData(2026, 11, 4) },
		// Hoje é 29/10: a janela vai de 31/10 (2 dias) a 08/11 (10 dias).
		"um dia de antecedência": func(c *CobrancaRecorrente, _ *Recorrencia) { c.Vencimento = NovaData(2026, 10, 30) },
		"onze dias":              func(c *CobrancaRecorrente, _ *Recorrencia) { c.Vencimento = NovaData(2026, 11, 9) },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			c, r := cobrValida(), recAprovada()
			estraga(&c, &r)
			if err := c.ValidarCriacao(r, agoraCobr); !errors.Is(err, ErrCobrancaRecorrenteInvalida) {
				t.Fatalf("err = %v; quer ErrCobrancaRecorrenteInvalida", err)
			}
		})
	}
}

func TestAntecedenciaUsaODiaDeBrasilia(t *testing.T) {
	// 01h de 30/10 em UTC ainda é 22h de 29/10 em Brasília: a janela começa em 31/10.
	agora := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	c := cobrValida()
	c.Vencimento = NovaData(2026, 10, 31)
	if err := c.ValidarCriacao(recAprovada(), agora); err != nil {
		t.Fatal(err)
	}
}

// vencida é a cobrança de 05/11 que não liquidou, vista em 05/11 às 15h.
func vencida() (CobrancaRecorrente, time.Time) {
	c := cobrValida()
	c.Tentativas = []Tentativa{{DataLiquidacao: c.Vencimento, Tipo: TipoAgendamento, Status: TentativaRejeitada}}
	return c, time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)
}

func TestRetentativaValida(t *testing.T) {
	c, agora := vencida()
	for _, nova := range []Data{NovaData(2026, 11, 6), NovaData(2026, 11, 12)} {
		if err := c.ValidarRetentativa(recAprovada(), nova, agora); err != nil {
			t.Errorf("%s: %v", nova, err)
		}
	}
	// Duas já pedidas: a terceira ainda cabe.
	c.Tentativas = append(c.Tentativas,
		Tentativa{DataLiquidacao: NovaData(2026, 11, 6), Tipo: TipoNovaTentativa, Status: TentativaRejeitada},
		Tentativa{DataLiquidacao: NovaData(2026, 11, 8), Tipo: TipoNovaTentativa, Status: TentativaRejeitada})
	if err := c.ValidarRetentativa(recAprovada(), NovaData(2026, 11, 10), time.Date(2026, 11, 9, 18, 0, 0, 0, time.UTC)); err != nil {
		t.Errorf("terceira: %v", err)
	}
}

func TestRetentativaRecusada(t *testing.T) {
	casos := map[string]struct {
		estraga func(*CobrancaRecorrente, *Recorrencia)
		nova    Data
	}{
		"política NAO_PERMITE":  {func(_ *CobrancaRecorrente, r *Recorrencia) { r.Politica = SemRetentativa }, NovaData(2026, 11, 6)},
		"outra recorrência":     {func(c *CobrancaRecorrente, _ *Recorrencia) { c.RecorrenciaID = "RN0000000020261029AAAAAAAAAAA" }, NovaData(2026, 11, 6)},
		"cobrança concluída":    {func(c *CobrancaRecorrente, _ *Recorrencia) { c.Status = CobrConcluida }, NovaData(2026, 11, 6)},
		"no próprio vencimento": {func(*CobrancaRecorrente, *Recorrencia) {}, NovaData(2026, 11, 5)},
		"antes do vencimento":   {func(*CobrancaRecorrente, *Recorrencia) {}, NovaData(2026, 11, 4)},
		"oito dias depois":      {func(*CobrancaRecorrente, *Recorrencia) {}, NovaData(2026, 11, 13)},
		"passa do fim":          {func(_ *CobrancaRecorrente, r *Recorrencia) { r.DataFinal = NovaData(2026, 11, 7) }, NovaData(2026, 11, 8)},
		// Semanal a partir de 05/10: o ciclo seguinte ao de 05/11 começa em 09/11.
		"cai no ciclo seguinte": {func(_ *CobrancaRecorrente, r *Recorrencia) { r.Periodicidade = Semanal }, NovaData(2026, 11, 9)},
		// Ciclos a partir de 04/10 e vencimento em 04/11: em 05/11, pedir 05/11 já é tarde.
		"pedida no próprio dia": {func(c *CobrancaRecorrente, r *Recorrencia) {
			r.DataInicial, c.Vencimento = NovaData(2026, 10, 4), NovaData(2026, 11, 4)
		}, NovaData(2026, 11, 5)},
		"data repetida": {func(c *CobrancaRecorrente, _ *Recorrencia) {
			c.Tentativas = append(c.Tentativas, Tentativa{DataLiquidacao: NovaData(2026, 11, 6), Tipo: TipoNovaTentativa, Status: TentativaAgendada})
		}, NovaData(2026, 11, 6)},
		"quarta data": {func(c *CobrancaRecorrente, _ *Recorrencia) {
			for _, dia := range []int{6, 7, 8} {
				c.Tentativas = append(c.Tentativas, Tentativa{DataLiquidacao: NovaData(2026, 11, dia), Tipo: TipoNovaTentativa, Status: TentativaRejeitada})
			}
		}, NovaData(2026, 11, 10)},
	}
	for nome, caso := range casos {
		t.Run(nome, func(t *testing.T) {
			c, agora := vencida()
			r := recAprovada()
			caso.estraga(&c, &r)
			if err := c.ValidarRetentativa(r, caso.nova, agora); !errors.Is(err, ErrRetentativaRecusada) {
				t.Fatalf("err = %v; quer ErrRetentativaRecusada", err)
			}
		})
	}
}

func TestRetentativasNaoContamOAgendamentoNemRepetidas(t *testing.T) {
	c := cobrValida()
	c.Tentativas = []Tentativa{
		{DataLiquidacao: NovaData(2026, 11, 5), Tipo: TipoAgendamento},
		{DataLiquidacao: NovaData(2026, 11, 6), Tipo: TipoNovaTentativa, Status: TentativaSolicitada},
		{DataLiquidacao: NovaData(2026, 11, 6), Tipo: TipoNovaTentativa, Status: TentativaAgendada},
	}
	if got := c.Retentativas(); len(got) != 1 || got[0] != NovaData(2026, 11, 6) {
		t.Fatalf("Retentativas() = %v", got)
	}
}

func TestPeriodicidadeSomar(t *testing.T) {
	casos := []struct {
		p    Periodicidade
		de   Data
		n    int
		quer Data
	}{
		{Semanal, NovaData(2026, 12, 28), 1, NovaData(2027, 1, 4)},
		{Mensal, NovaData(2026, 1, 31), 1, NovaData(2026, 2, 28)},
		{Mensal, NovaData(2028, 1, 31), 1, NovaData(2028, 2, 29)},
		{Mensal, NovaData(2026, 1, 31), 2, NovaData(2026, 3, 31)},
		{Mensal, NovaData(2026, 11, 15), 3, NovaData(2027, 2, 15)},
		{Trimestral, NovaData(2026, 11, 30), 1, NovaData(2027, 2, 28)},
		{Semestral, NovaData(2026, 8, 31), 1, NovaData(2027, 2, 28)},
		{Anual, NovaData(2028, 2, 29), 1, NovaData(2029, 2, 28)},
	}
	for _, c := range casos {
		if got := c.p.Somar(c.de, c.n); got != c.quer {
			t.Errorf("%s.Somar(%s, %d) = %s, quer %s", c.p, c.de, c.n, got, c.quer)
		}
	}
}

func TestInicioDoCicloSeguinte(t *testing.T) {
	r := recAprovada() // mensal a partir de 05/10/2026
	if got := r.InicioDoCicloSeguinte(NovaData(2026, 11, 5)); got != NovaData(2026, 12, 5) {
		t.Errorf("no início do ciclo: %s", got)
	}
	if got := r.InicioDoCicloSeguinte(NovaData(2026, 11, 20)); got != NovaData(2026, 12, 5) {
		t.Errorf("no meio do ciclo: %s", got)
	}
	r.Periodicidade = "QUINZENAL"
	if got := r.InicioDoCicloSeguinte(NovaData(2026, 11, 5)); !got.Zero() {
		t.Errorf("periodicidade inventada: %s", got)
	}
}

func TestParametrosEm(t *testing.T) {
	defer func(original []Parametros) { TabelaParametros = original }(TabelaParametros)
	TabelaParametros = []Parametros{
		{Fonte: "v1", Desde: NovaData(2026, 1, 1), AntecedenciaMinimaDias: 2},
		{Fonte: "v2", Desde: NovaData(2027, 1, 1), AntecedenciaMinimaDias: 3},
	}
	casos := map[Data]string{
		NovaData(2025, 6, 1):   "v1", // antes da primeira linha vale a primeira
		NovaData(2026, 12, 31): "v1",
		NovaData(2027, 1, 1):   "v2",
		NovaData(2030, 1, 1):   "v2",
	}
	for d, quer := range casos {
		if got := ParametrosEm(d).Fonte; got != quer {
			t.Errorf("ParametrosEm(%s) = %s, quer %s", d, got, quer)
		}
	}
}

func TestTabelaParametrosEmOrdemEComFonte(t *testing.T) {
	for i, p := range TabelaParametros {
		if p.Fonte == "" || p.Desde.Zero() {
			t.Errorf("linha %d sem fonte ou data do normativo", i)
		}
		if i > 0 && !p.Desde.Depois(TabelaParametros[i-1].Desde) {
			t.Errorf("linha %d fora de ordem", i)
		}
	}
	// Os valores de partida da RN-43.
	p := ParametrosEm(NovaData(2026, 10, 29))
	if p.PendenciaMaximaSolicitacao != 30*24*time.Hour || p.AntecedenciaMinimaDias != 2 || p.AntecedenciaMaximaDias != 10 ||
		p.RetentativasMaximas != 3 || p.JanelaRetentativaDias != 7 {
		t.Errorf("parâmetros de 29/10/2026 = %+v", p)
	}
}
