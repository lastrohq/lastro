package oauth_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/platform/oauth"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

var inicio = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

var _ oauth.Relogio = ports.Clock(nil)

// banco simula o endpoint de token: conta as buscas e devolve tokens numerados.
type banco struct {
	relogio *fake.Clock
	vida    time.Duration
	buscas  atomic.Int64
}

func (b *banco) buscar(ctx context.Context) (oauth.Token, error) {
	n := b.buscas.Add(1)
	agora := b.relogio.Now()
	return oauth.Token{Valor: fmt.Sprintf("token-%d", n), ObtidoEm: agora, ExpiraEm: agora.Add(b.vida)}, nil
}

func novoBanco(vida time.Duration) (*banco, *oauth.Cache) {
	relogio := fake.NewClock(inicio)
	return &banco{relogio: relogio, vida: vida}, oauth.NovoCache(relogio)
}

func TestReutilizaTokenAteExpirar(t *testing.T) {
	b, c := novoBanco(time.Hour)
	for range 3 {
		tok, err := c.Token(t.Context(), "loja", b.buscar)
		if err != nil || tok.Valor != "token-1" {
			t.Fatalf("Token = %q, %v; quer token-1", tok.Valor, err)
		}
	}
	if n := b.buscas.Load(); n != 1 {
		t.Fatalf("buscas = %d; quer 1", n)
	}
}

func TestChamadasConcorrentesFazemUmaBusca(t *testing.T) {
	relogio := fake.NewClock(inicio)
	c := oauth.NovoCache(relogio)
	liberar := make(chan struct{})
	var buscas atomic.Int64
	buscar := func(ctx context.Context) (oauth.Token, error) {
		buscas.Add(1)
		<-liberar // segura a busca até todos estarem esperando
		return oauth.Token{Valor: "unico", ObtidoEm: inicio, ExpiraEm: inicio.Add(time.Hour)}, nil
	}

	const n = 50
	var wg sync.WaitGroup
	erros := make(chan error, n)
	for range n {
		wg.Go(func() {
			tok, err := c.Token(t.Context(), "loja", buscar)
			if err == nil && tok.Valor != "unico" {
				err = fmt.Errorf("token %q", tok.Valor)
			}
			erros <- err
		})
	}
	time.Sleep(50 * time.Millisecond) // dá tempo de as 50 chamadas chegarem
	close(liberar)
	wg.Wait()
	close(erros)
	for err := range erros {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := buscas.Load(); got != 1 {
		t.Fatalf("buscas = %d com %d chamadas concorrentes; quer 1", got, n)
	}
}

func TestRenovaAntesDeExpirar(t *testing.T) {
	b, c := novoBanco(time.Hour)
	if _, err := c.Token(t.Context(), "loja", b.buscar); err != nil {
		t.Fatal(err)
	}

	b.relogio.Avancar(time.Hour - 61*time.Second) // faltam 61 s: ainda serve
	tok, _ := c.Token(t.Context(), "loja", b.buscar)
	if tok.Valor != "token-1" {
		t.Fatalf("com 61 s de vida, Token = %q; quer token-1", tok.Valor)
	}

	b.relogio.Avancar(2 * time.Second) // faltam 59 s: dentro da antecedência de 60 s
	tok, _ = c.Token(t.Context(), "loja", b.buscar)
	if tok.Valor != "token-2" {
		t.Fatalf("com 59 s de vida, Token = %q; quer token-2 (renovado antes de expirar)", tok.Valor)
	}
}

func TestTokenDeVidaCurtaRenovaNaMetade(t *testing.T) {
	b, c := novoBanco(30 * time.Second) // vida menor que a antecedência de 60 s
	_, _ = c.Token(t.Context(), "loja", b.buscar)

	b.relogio.Avancar(14 * time.Second)
	if tok, _ := c.Token(t.Context(), "loja", b.buscar); tok.Valor != "token-1" {
		t.Fatalf("aos 14 s de 30, Token = %q; quer token-1", tok.Valor)
	}
	b.relogio.Avancar(2 * time.Second)
	if tok, _ := c.Token(t.Context(), "loja", b.buscar); tok.Valor != "token-2" {
		t.Fatalf("aos 16 s de 30, Token = %q; quer token-2", tok.Valor)
	}
}

func TestContasTemTokensSeparados(t *testing.T) {
	b, c := novoBanco(time.Hour)
	a, _ := c.Token(t.Context(), "loja-a", b.buscar)
	o, _ := c.Token(t.Context(), "loja-b", b.buscar)
	if a.Valor == o.Valor {
		t.Fatalf("duas contas com o mesmo token %q", a.Valor)
	}
}

func TestErroNaoFicaGuardado(t *testing.T) {
	c := oauth.NovoCache(fake.NewClock(inicio))
	falhar := true
	buscar := func(ctx context.Context) (oauth.Token, error) {
		if falhar {
			return oauth.Token{}, errors.New("banco fora do ar")
		}
		return oauth.Token{Valor: "ok", ObtidoEm: inicio, ExpiraEm: inicio.Add(time.Hour)}, nil
	}
	if _, err := c.Token(t.Context(), "loja", buscar); err == nil {
		t.Fatal("quer o erro da busca")
	}
	falhar = false
	if tok, err := c.Token(t.Context(), "loja", buscar); err != nil || tok.Valor != "ok" {
		t.Fatalf("depois da falha, Token = %q, %v; quer ok", tok.Valor, err)
	}
}

func TestQuemDesisteNaoCancelaABuscaDosOutros(t *testing.T) {
	c := oauth.NovoCache(fake.NewClock(inicio))
	liberar := make(chan struct{})
	var buscas atomic.Int64
	buscar := func(ctx context.Context) (oauth.Token, error) {
		buscas.Add(1)
		select {
		case <-liberar:
			return oauth.Token{Valor: "ok", ObtidoEm: inicio, ExpiraEm: inicio.Add(time.Hour)}, nil
		case <-ctx.Done():
			return oauth.Token{}, ctx.Err()
		}
	}

	ctx, cancelar := context.WithCancel(t.Context())
	desistiu := make(chan error)
	go func() {
		_, err := c.Token(ctx, "loja", buscar)
		desistiu <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancelar()
	if err := <-desistiu; !errors.Is(err, context.Canceled) {
		t.Fatalf("quem desistiu recebeu %v; quer context.Canceled", err)
	}

	esperou := make(chan oauth.Token)
	go func() {
		tok, _ := c.Token(t.Context(), "loja", buscar)
		esperou <- tok
	}()
	time.Sleep(20 * time.Millisecond)
	close(liberar)
	if tok := <-esperou; tok.Valor != "ok" {
		t.Fatalf("quem esperou recebeu %q; quer ok", tok.Valor)
	}
	if n := buscas.Load(); n != 1 {
		t.Fatalf("buscas = %d; quer 1 (a busca continuou depois da desistência)", n)
	}
}

func TestBuscaTemTimeout(t *testing.T) {
	c := oauth.NovoCache(fake.NewClock(inicio), oauth.ComTimeout(10*time.Millisecond))
	buscar := func(ctx context.Context) (oauth.Token, error) {
		<-ctx.Done()
		return oauth.Token{}, ctx.Err()
	}
	if _, err := c.Token(t.Context(), "loja", buscar); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Token = %v; quer DeadlineExceeded", err)
	}
}

func TestInvalidarSoDescartaOTokenRecusado(t *testing.T) {
	b, c := novoBanco(time.Hour)
	velho, _ := c.Token(t.Context(), "loja", b.buscar)
	c.Invalidar("loja", velho)
	novo, _ := c.Token(t.Context(), "loja", b.buscar)
	if novo.Valor != "token-2" {
		t.Fatalf("depois de invalidar, Token = %q; quer token-2", novo.Valor)
	}

	c.Invalidar("loja", velho) // outra chamada atrasada recusando o token velho
	if tok, _ := c.Token(t.Context(), "loja", b.buscar); tok.Valor != "token-2" {
		t.Fatalf("invalidar o velho descartou o novo: Token = %q", tok.Valor)
	}
}

func TestAntecedenciaConfiguravel(t *testing.T) {
	relogio := fake.NewClock(inicio)
	b := &banco{relogio: relogio, vida: time.Hour}
	c := oauth.NovoCache(relogio, oauth.ComAntecedencia(5*time.Minute))
	_, _ = c.Token(t.Context(), "loja", b.buscar)
	relogio.Avancar(time.Hour - 4*time.Minute)
	if tok, _ := c.Token(t.Context(), "loja", b.buscar); tok.Valor != "token-2" {
		t.Fatalf("com 4 min de vida e antecedência de 5, Token = %q; quer token-2", tok.Valor)
	}
}
