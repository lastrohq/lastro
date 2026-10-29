package domain

import (
	"errors"
	"fmt"
	"time"
	_ "time/tzdata" // a imagem distroless não tem o banco de fusos; ele vai dentro do binário
)

// Brasilia é o fuso das datas do Pix: vencimento e "hoje" são avaliados nele.
var Brasilia = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err) // time/tzdata está embutido: só falha se o import sumir
	}
	return loc
}()

// ErrDataInvalida indica uma data fora do formato AAAA-MM-DD.
var ErrDataInvalida = errors.New("data inválida")

// Data é uma data civil (AAAA-MM-DD), sem hora nem fuso.
type Data struct {
	t time.Time // meia-noite UTC do dia; UTC evita surpresa com horário de verão
}

// ParseData lê AAAA-MM-DD.
func ParseData(s string) (Data, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Data{}, fmt.Errorf("%w: %q (use AAAA-MM-DD)", ErrDataInvalida, s)
	}
	return Data{t: t}, nil
}

// DataDe devolve a data do instante t no fuso de Brasília.
func DataDe(t time.Time) Data {
	a, m, d := t.In(Brasilia).Date()
	return Data{t: time.Date(a, m, d, 0, 0, 0, 0, time.UTC)}
}

// NovaData monta a data a partir de ano, mês e dia.
func NovaData(ano int, mes time.Month, dia int) Data {
	return Data{t: time.Date(ano, mes, dia, 0, 0, 0, 0, time.UTC)}
}

func (d Data) String() string { return d.t.Format(time.DateOnly) }

// Zero informa se a data não foi preenchida.
func (d Data) Zero() bool { return d.t.IsZero() }

// SomarDias devolve a data n dias corridos depois.
func (d Data) SomarDias(n int) Data { return Data{t: d.t.AddDate(0, 0, n)} }

// Antes informa se d vem antes de o.
func (d Data) Antes(o Data) bool { return d.t.Before(o.t) }

// Depois informa se d vem depois de o.
func (d Data) Depois(o Data) bool { return d.t.After(o.t) }

// Tempo devolve a data como time.Time (meia-noite UTC), para quem precisa do tipo da stdlib.
func (d Data) Tempo() time.Time { return d.t }
