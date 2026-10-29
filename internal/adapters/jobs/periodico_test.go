package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPeriodicoRodaLogoERepeteAtePararSemSobrepor(t *testing.T) {
	ctx, cancelar := context.WithCancel(context.Background())
	var rodadas, simultaneas, pico atomic.Int32
	fim := make(chan struct{})
	go func() {
		Periodico(ctx, "teste", 10*time.Millisecond, func(context.Context) error {
			n := simultaneas.Add(1)
			if n > pico.Load() {
				pico.Store(n)
			}
			time.Sleep(5 * time.Millisecond)
			simultaneas.Add(-1)
			if rodadas.Add(1) == 3 {
				cancelar()
			}
			return errors.New("falha não interrompe o laço")
		})
		close(fim)
	}()
	select {
	case <-fim:
	case <-time.After(2 * time.Second):
		t.Fatal("Periodico não parou com o contexto cancelado")
	}
	if rodadas.Load() < 3 || pico.Load() != 1 {
		t.Fatalf("rodadas=%d pico de simultâneas=%d", rodadas.Load(), pico.Load())
	}
}
