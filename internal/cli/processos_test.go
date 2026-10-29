package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/ports"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

// pedir faz uma chamada HTTP ao serve no ar e devolve o status.
type pedir func(metodo, caminho, idem, corpo string, saida any) int

// lastroEmProcessos roda o Lastro como o binário roda: cada comando monta um
// sandbox novo, lendo o store, então o que o sandbox lembra entre processos é
// só o que o Lastro gravou.
type lastroEmProcessos struct {
	t       *testing.T
	amb     Ambiente
	out     *bytes.Buffer
	relogio *fake.Clock
	base    string
	token   string
}

// novoLastroEmProcessos cria a conta "loja" no sandbox e uma API key dela com os escopos dados.
func novoLastroEmProcessos(t *testing.T, escopos ...string) *lastroEmProcessos {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	env := envComBanco(t)
	env["LASTRO_ADDR"], env["LASTRO_CONCILIACAO_INTERVALO"], env["LASTRO_WEBHOOK_INTERVALO"] = addr, "0", "0"
	amb, out := ambiente(t, env)
	relogio := fake.NewClock(time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC))
	amb.Clock = relogio
	amb.MontarPSPs = func(f Fontes) ports.Registry { return psp.NewRegistry(sandbox.New(relogio, sandbox.ComStore(f.Store))) }
	p := &lastroEmProcessos{t: t, amb: amb, out: out, relogio: relogio, base: "http://" + addr}

	p.rodar("conta", "criar", "--id", "loja", "--nome", "Loja", "--psp", "sandbox", "--chave", "loja@example.com")
	args := []string{"apikey", "criar", "--nome", "teste", "--conta", "loja"}
	for _, e := range escopos {
		args = append(args, "--escopo", e)
	}
	p.rodar(args...)
	p.token = regexp.MustCompile(`lst_\S+`).FindString(out.String())
	return p
}

// rodar executa um comando da CLI e devolve a saída dele.
func (p *lastroEmProcessos) rodar(args ...string) string {
	p.t.Helper()
	p.out.Reset()
	if err := Run(context.Background(), args, p.amb); err != nil {
		p.t.Fatalf("%v: %v (%s)", args, err, p.out)
	}
	return p.out.String()
}

// servir sobe um serve, roda as chamadas e para, como um processo que termina.
func (p *lastroEmProcessos) servir(chamadas func(pedir)) {
	p.t.Helper()
	ctx := context.Background()
	servindo, parar := context.WithCancel(ctx)
	feito := make(chan error, 1)
	go func() { feito <- Run(servindo, []string{"serve"}, p.amb) }()
	chamar := func(metodo, caminho, idem, corpo string, saida any) int {
		p.t.Helper()
		req, _ := http.NewRequestWithContext(ctx, metodo, p.base+caminho, strings.NewReader(corpo))
		req.Header.Set("Authorization", "Bearer "+p.token)
		req.Header.Set("Content-Type", "application/json")
		if idem != "" {
			req.Header.Set("Idempotency-Key", idem)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		if saida != nil {
			_ = json.Unmarshal(b, saida)
		}
		return resp.StatusCode
	}
	for i := 0; chamar(http.MethodGet, "/health", "", "", nil) != http.StatusOK; i++ {
		if i > 100 {
			p.t.Fatal("o serve não subiu")
		}
		time.Sleep(20 * time.Millisecond)
	}
	chamadas(chamar)
	parar()
	if err := <-feito; err != nil {
		p.t.Fatalf("serve: %v", err)
	}
}

// conciliar roda `lastro conciliar` e confere que a saída contém quer.
func (p *lastroEmProcessos) conciliar(quer string) {
	p.t.Helper()
	if saida := p.rodar("conciliar"); !strings.Contains(saida, quer) {
		p.t.Fatalf("conciliar: %s; quer %q", saida, quer)
	}
}
