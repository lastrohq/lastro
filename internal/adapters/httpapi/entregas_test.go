package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/lastrohq/lastro/internal/adapters/httpapi/gen"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
)

type enviadorOK struct{ chamadas int }

func (e *enviadorOK) Enviar(context.Context, string, map[string]string, []byte) (int, error) {
	e.chamadas++
	return http.StatusNoContent, nil
}

// naDLQ cria um destino e uma entrega que desistiu, como depois de 24 h de falhas.
func (a *api) naDLQ(saida app.Saida, eventoID string) {
	a.t.Helper()
	ctx := context.Background()
	d, _, err := saida.ConfigurarDestino(ctx, "loja", "https://loja.example.com/webhooks/lastro")
	if err != nil {
		a.t.Fatal(err)
	}
	agora := a.clock.Now()
	e := domain.Entrega{EventoID: eventoID, DestinoID: d.ID, ContaID: "loja", Tipo: "cobranca.concluida", Corpo: []byte(`{}`),
		CriadaEm: agora, Status: domain.EntregaPendente, ProximaEm: agora, AtualizadaEm: agora}
	if err := a.store.Entregas().Criar(ctx, e); err != nil {
		a.t.Fatal(err)
	}
	e.Status, e.Tentativas, e.UltimoHTTP, e.UltimoErro = domain.EntregaDLQ, 9, http.StatusInternalServerError, "status 500"
	if err := a.store.Entregas().Atualizar(ctx, e); err != nil {
		a.t.Fatal(err)
	}
}

// O reenvio pela API devolve a entrega à fila e a próxima tentativa fica registrada (#36).
func TestReenviarDaDLQGeraNovaTentativa(t *testing.T) {
	a := novaAPI(t)
	envio := &enviadorOK{}
	saida := app.Saida{Store: a.store, Vault: a.vault, Clock: a.clock, Enviador: envio}
	a.naDLQ(saida, "evt_1")

	var lista gen.ListarEntregas200JSONResponse
	if st := a.chamar(http.MethodGet, "/lastro/v1/webhooks/entregas", "", nil, &lista); st != http.StatusOK || len(lista.Entregas) != 1 {
		t.Fatalf("DLQ: %d %+v", st, lista)
	}
	if e := lista.Entregas[0]; e.EventoId != "evt_1" || e.Tentativas != 9 || e.UltimoHttp == nil || *e.UltimoHttp != 500 {
		t.Fatalf("entrega na DLQ: %+v", e)
	}

	var r gen.ReenviarEntregas200JSONResponse
	if st := a.chamar(http.MethodPost, "/lastro/v1/webhooks/entregas/evt_1/reenviar", "", nil, &r); st != http.StatusOK || r.Reenviadas != 1 {
		t.Fatalf("reenviar: %d %+v", st, r)
	}
	if _, err := saida.Entregar(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.chamar(http.MethodGet, "/lastro/v1/webhooks/entregas?status=entregue", "", nil, &lista)
	if len(lista.Entregas) != 1 || lista.Entregas[0].Tentativas != 1 || envio.chamadas != 1 {
		t.Fatalf("depois do reenvio: %+v (%d envios)", lista.Entregas, envio.chamadas)
	}
	if st := a.chamar(http.MethodPost, "/lastro/v1/webhooks/entregas/evt_1/reenviar", "", nil, nil); st != http.StatusNotFound {
		t.Fatalf("reenviar de novo, sem nada na DLQ: %d", st)
	}
}

func TestEntregasRespeitamEscopoEConta(t *testing.T) {
	a := novaAPI(t)
	a.naDLQ(app.Saida{Store: a.store, Vault: a.vault, Clock: a.clock}, "evt_1")

	semEscopo := map[string]string{"Authorization": "Bearer " + a.novoToken("", domain.EscopoCobRead)}
	if st := a.chamar(http.MethodGet, "/lastro/v1/webhooks/entregas", "", semEscopo, nil); st != http.StatusForbidden {
		t.Fatalf("sem webhook:admin: %d", st)
	}
	if err := a.store.Contas().Criar(context.Background(), domain.Conta{ID: "outra", Nome: "Outra", PSP: "sandbox", ChavePix: "outra@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	outraConta := map[string]string{"Authorization": "Bearer " + a.novoToken("outra", domain.EscopoWebhookAdmin)}
	var lista gen.ListarEntregas200JSONResponse
	if st := a.chamar(http.MethodGet, "/lastro/v1/webhooks/entregas", "", outraConta, &lista); st != http.StatusOK || len(lista.Entregas) != 0 {
		t.Fatalf("chave de outra conta viu: %d %+v", st, lista.Entregas)
	}
	if st := a.chamar(http.MethodPost, "/lastro/v1/webhooks/entregas/evt_1/reenviar", "", outraConta, nil); st != http.StatusNotFound {
		t.Fatalf("chave de outra conta reenviou: %d", st)
	}
	daLoja := map[string]string{"Authorization": "Bearer " + a.novoToken("loja", domain.EscopoWebhookAdmin)}
	if st := a.chamar(http.MethodPost, "/lastro/v1/webhooks/entregas/evt_1/reenviar", "", daLoja, nil); st != http.StatusOK {
		t.Fatalf("chave da loja: %d", st)
	}
}
