package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/psp/sandbox"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
)

// TestCobrComRetentativaNoSandbox é o critério da #32, entre processos como
// no binário: cada comando monta um sandbox novo, lendo o store. Uma
// recorrência de R$ 0,55 é aprovada, a cobrança falha no vencimento por falta
// de saldo, a retentativa é paga, e cada tentativa vira um evento.
func TestCobrComRetentativaNoSandbox(t *testing.T) {
	p := novoLastroEmProcessos(t, "rec:write", "rec:read", "cobr:write", "cobr:read")
	relogio, servir, conciliar := p.relogio, p.servir, p.conciliar
	ctx := context.Background()
	const txid = "cobrcli000000000000000000001"
	var idRec string

	// 1. A recorrência de R$ 0,55 é criada, pedida e aprovada pelo pagador.
	servir(func(pedir pedir) {
		var rec struct {
			IDRec string `json:"idRec"`
		}
		corpo := `{"vinculo":{"contrato":"c1","devedor":{"cpf":"12345678909","nome":"Fulano"}},"calendario":{"dataInicial":"2026-11-01","periodicidade":"MENSAL"},"valor":{"valorRec":"0.55"},"politicaRetentativa":"PERMITE_3R_7D"}`
		if st := pedir(http.MethodPost, "/rec", "rec-1", corpo, &rec); st != http.StatusCreated {
			t.Fatalf("criar recorrência: %d", st)
		}
		idRec = rec.IDRec
		sol := fmt.Sprintf(`{"idRec":%q,"calendario":{"dataExpiracaoSolicitacao":%q},"destinatario":{"conta":"1","ispbParticipante":"18236120"}}`, idRec, relogio.Now().Add(24*time.Hour).Format(time.RFC3339))
		if st := pedir(http.MethodPost, "/solicrec", "sol-1", sol, nil); st != http.StatusCreated {
			t.Fatalf("solicitar: %d", st)
		}
	})
	relogio.Avancar(sandbox.AtrasoPadrao)
	conciliar("1 recorrências atualizadas")

	// 2. A cobrança de novembro.
	servir(func(pedir pedir) {
		corpo := fmt.Sprintf(`{"idRec":%q,"calendario":{"dataDeVencimento":"2026-11-05"},"valor":{"original":"0.55"}}`, idRec)
		if st := pedir(http.MethodPut, "/cobr/"+txid, "", corpo, nil); st != http.StatusCreated {
			t.Fatalf("criar cobrança: %d", st)
		}
	})

	// 3. A tentativa do vencimento falha por falta de saldo; nenhum aviso chega,
	// e a conciliação encontra.
	relogio.Avancar(2 * sandbox.AtrasoPadrao)
	conciliar("1 cobranças recorrentes atualizadas")

	// 4. No dia do vencimento, o recebedor pede uma nova data.
	relogio.Avancar(time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC).Sub(relogio.Now()))
	servir(func(pedir pedir) {
		var c struct {
			Status     string `json:"status"`
			Tentativas []struct {
				Tipo, Status string
			} `json:"tentativas"`
		}
		if st := pedir(http.MethodPost, "/cobr/"+txid+"/retentativa/2026-11-07", "", "", &c); st != http.StatusCreated {
			t.Fatalf("retentativa: %d", st)
		}
		if c.Status != "ATIVA" || len(c.Tentativas) != 2 || c.Tentativas[0].Status != "REJEITADA" || c.Tentativas[1].Tipo != "NTAG" {
			t.Fatalf("depois de pedir a retentativa: %+v", c)
		}
	})

	// 5. A retentativa é paga, e a conciliação fecha a cobrança.
	relogio.Avancar(sandbox.AtrasoPadrao)
	conciliar("1 cobranças recorrentes atualizadas")
	conciliar("0 cobranças recorrentes atualizadas")

	contas, fechar, err := abrir(ctx, p.amb)
	if err != nil {
		t.Fatal(err)
	}
	defer fechar()
	c, err := contas.Store.CobrancasRecorrentes().BuscarPorTxid(ctx, txid)
	if err != nil || c.Status != domain.CobrConcluida || len(c.Tentativas) != 2 || c.Tentativas[1].Status != domain.TentativaPaga || c.Tentativas[1].EndToEndID == "" {
		t.Fatalf("no fim: %+v, %v", c, err)
	}
	// Um evento por tentativa: a do vencimento rejeitada, a nova agendada e a nova paga.
	evs, _ := contas.Store.Outbox().NaoDistribuidos(ctx, 100)
	var tentativas []string
	for _, e := range evs {
		if e.Tipo != domain.EventoCobrRTentativa {
			continue
		}
		var d app.DadosCobrancaRecorrente
		_ = json.Unmarshal(e.Dados, &d)
		tentativas = append(tentativas, d.Tentativa.Tipo+" "+d.Tentativa.Status)
	}
	if strings.Join(tentativas, ", ") != "AGND REJEITADA, NTAG AGENDADA, NTAG PAGA" {
		t.Fatalf("eventos de tentativa: %v", tentativas)
	}
}
