package cli

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp/sandbox"
)

// TestDevolucaoEntreProcessos: a cob é paga no sandbox, a conciliação grava o
// Pix, e a devolução é pedida pela API. Num processo seguinte, com o sandbox
// de memória vazia, o par repetido devolve a mesma devolução e o saldo
// continua descontado.
func TestDevolucaoEntreProcessos(t *testing.T) {
	p := novoVintemEmProcessos(t, "cob:write", "pix:read", "devolucao:write")
	const txid = "devcli0000000000000000000001"
	p.servir(func(pedir pedir) {
		corpo := `{"calendario":{"expiracao":3600},"valor":{"original":"50.00"},"chave":"loja@example.com"}`
		if st := pedir(http.MethodPut, "/cob/"+txid, "", corpo, nil); st != http.StatusCreated {
			t.Fatalf("criar cob: %d", st)
		}
	})
	p.relogio.Avancar(sandbox.AtrasoPadrao + time.Second) // a janela da conciliação não inclui o agora
	p.conciliar("1 Pix recuperados")

	contas, fechar, err := abrir(context.Background(), p.amb)
	if err != nil {
		t.Fatal(err)
	}
	pix, err := contas.Store.Pix().ListarPorTxid(context.Background(), txid)
	fechar()
	if err != nil || len(pix) != 1 {
		t.Fatalf("Pix da cob: %+v, %v", pix, err)
	}
	caminho := func(id string) string { return "/pix/" + pix[0].EndToEndID + "/devolucao/" + id }

	type devolucao struct {
		Status string `json:"status"`
		RtrID  string `json:"rtrId"`
	}
	var primeira devolucao
	p.servir(func(pedir pedir) {
		if st := pedir(http.MethodPut, caminho("dev1"), "", `{"valor":"30.00"}`, &primeira); st != http.StatusCreated || primeira.Status != "DEVOLVIDO" || primeira.RtrID == "" {
			t.Fatalf("devolver: %d %+v", st, primeira)
		}
	})
	p.servir(func(pedir pedir) {
		var repetida, lida devolucao
		if st := pedir(http.MethodPut, caminho("dev1"), "", `{"valor":"30.00"}`, &repetida); st != http.StatusCreated || repetida != primeira {
			t.Fatalf("par repetido em outro processo: %d %+v", st, repetida)
		}
		if st := pedir(http.MethodGet, caminho("dev1"), "", "", &lida); st != http.StatusOK || lida != primeira {
			t.Fatalf("consultar: %d %+v", st, lida)
		}
		if st := pedir(http.MethodPut, caminho("dev2"), "", `{"valor":"20.01"}`, nil); st != http.StatusBadRequest {
			t.Fatalf("passar do saldo: %d", st)
		}
		if st := pedir(http.MethodPut, caminho("dev2"), "", `{"valor":"20.00"}`, nil); st != http.StatusCreated {
			t.Fatalf("o resto do saldo: %d", st)
		}
	})
}
