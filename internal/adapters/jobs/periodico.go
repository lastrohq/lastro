// Package jobs roda tarefas periódicas dentro do processo do Lastro, sem fila
// externa (ADR-005): conciliação hoje, entrega de webhooks e limpezas depois.
package jobs

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Periodico roda fn ao começar e depois a cada intervalo, até ctx acabar.
// As rodadas nunca se sobrepõem: a próxima espera a anterior terminar. Um
// atraso aleatório de até 10% do intervalo evita que várias instâncias
// batam no banco no mesmo segundo.
func Periodico(ctx context.Context, nome string, intervalo time.Duration, fn func(context.Context) error) {
	for {
		inicio := time.Now()
		if err := fn(ctx); err != nil && ctx.Err() == nil {
			slog.ErrorContext(ctx, "job falhou", "job", nome, "erro", err)
		}
		slog.DebugContext(ctx, "job rodou", "job", nome, "duracao", time.Since(inicio))
		espera := intervalo + time.Duration(rand.Int64N(int64(intervalo)/10+1)) //nolint:gosec // G404: jitter, não segurança
		select {
		case <-ctx.Done():
			return
		case <-time.After(espera):
		}
	}
}
