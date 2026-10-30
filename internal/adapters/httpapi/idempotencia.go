package httpapi

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/app"
)

// limiteCorpo é o maior corpo aceito numa requisição com Idempotency-Key.
const limiteCorpo = 1 << 20

// idempotente aplica o Idempotency-Key (RN-11) a POST, PUT e PATCH. Roda
// depois da autenticação: a chave pertence à API key que a enviou.
func idempotente(idem app.Idempotencia) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			chave := r.Header.Get("Idempotency-Key")
			if idem.Store == nil || chave == "" || (r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch) {
				next.ServeHTTP(w, r)
				return
			}
			corpo, err := io.ReadAll(io.LimitReader(r.Body, limiteCorpo+1))
			if err != nil || len(corpo) > limiteCorpo {
				escreverProblema(w, gen.Problema{Status: http.StatusRequestEntityTooLarge, Type: tipoErro("CorpoGrandeDemais"), Title: "O corpo passa de 1 MiB"})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(corpo))

			h := sha256.New()
			h.Write([]byte(r.Method + " " + r.URL.Path + "\n"))
			h.Write(corpo)
			hash := h.Sum(nil)
			dono := app.AcessoDe(r.Context()).APIKeyID

			guardada, err := idem.Iniciar(r.Context(), dono, chave, hash)
			switch {
			case errors.Is(err, app.ErrIdempotenciaDivergente):
				escreverProblema(w, gen.Problema{Status: http.StatusConflict, Type: tipoErro("IdempotencyKeyReutilizada"), Title: "A Idempotency-Key já foi usada com outra requisição", Detail: ptr(err.Error())})
				return
			case errors.Is(err, app.ErrIdempotenciaEmAndamento):
				w.Header().Set("Retry-After", "1")
				escreverProblema(w, gen.Problema{Status: http.StatusConflict, Type: tipoErro("RequisicaoEmAndamento"), Title: "A primeira requisição com essa Idempotency-Key ainda não terminou", Detail: ptr(err.Error())})
				return
			case err != nil:
				escreverProblema(w, problemaDe(r.Context(), err))
				return
			case guardada != nil:
				w.Header().Set("Content-Type", guardada.ContentType)
				w.Header().Set("Idempotent-Replayed", "true")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(guardada.Status)
				_, _ = w.Write(guardada.Corpo) //nolint:gosec // G705: é a resposta JSON que o próprio Vintém gerou e guardou, com o Content-Type original

				return
			}

			gravador := &gravadorResposta{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(gravador, r)
			// A resposta já foi para o cliente; falhar ao guardar só custa a próxima repetição.
			_ = idem.Concluir(r.Context(), dono, chave, hash, app.RespostaGuardada{
				Status: gravador.status, ContentType: w.Header().Get("Content-Type"), Corpo: gravador.corpo.Bytes(),
			})
		})
	}
}

// gravadorResposta repassa a resposta ao cliente e guarda uma cópia.
type gravadorResposta struct {
	http.ResponseWriter
	status int
	corpo  bytes.Buffer
}

func (g *gravadorResposta) WriteHeader(status int) {
	g.status = status
	g.ResponseWriter.WriteHeader(status)
}

func (g *gravadorResposta) Write(b []byte) (int, error) {
	g.corpo.Write(b)
	return g.ResponseWriter.Write(b)
}
