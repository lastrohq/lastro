package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Saida entrega os eventos do outbox à aplicação do cliente, assinados no
// padrão Standard Webhooks, com retry por 24 h e DLQ (RN-32, ADR-005).
type Saida struct {
	Store    ports.Store
	Vault    ports.Vault
	Clock    ports.Clock
	Enviador ports.Enviador
	// Paralelas é quantas entregas correm ao mesmo tempo; zero usa 8. Um destino
	// lento (até 10 s de timeout) não pode travar a fila dos outros.
	Paralelas int
}

// ConfigurarDestino cadastra uma URL para a conta (ou para todas, com contaID
// vazio) e devolve o segredo whsec_, que só aparece aqui.
func (s Saida) ConfigurarDestino(ctx context.Context, contaID, endereco string) (domain.Destino, string, error) {
	u, err := url.Parse(endereco)
	local := err == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")
	esquemaOK := err == nil && (u.Scheme == "https" || (u.Scheme == "http" && local))
	if err != nil || u.Host == "" || !esquemaOK {
		return domain.Destino{}, "", &ErroValidacao{Violacoes: []Violacao{{Propriedade: "url", Razao: "use https:// (http só para localhost)", Valor: endereco}}}
	}
	chave, whsec := domain.NovoSegredoWebhook()
	cifrado, err := s.Vault.Cifrar(ctx, chave)
	if err != nil {
		return domain.Destino{}, "", fmt.Errorf("cifrando segredo: %w", err)
	}
	d := domain.Destino{ID: "dst_" + novoID()[:16], ContaID: contaID, URL: endereco, SegredoCifrado: cifrado, CriadoEm: s.Clock.Now()}
	if err := s.Store.Destinos().Criar(ctx, d); err != nil {
		return domain.Destino{}, "", err
	}
	return d, whsec, nil
}

// envelope é o corpo enviado: o formato recomendado pelo Standard Webhooks.
type envelope struct {
	Type      domain.TipoEvento `json:"type"`
	Timestamp time.Time         `json:"timestamp"`
	Data      json.RawMessage   `json:"data"`
}

// Distribuir cria uma entrega por destino para cada evento novo do outbox.
// Evento sem destino é marcado e não volta: destino criado depois não recebe o passado.
func (s Saida) Distribuir(ctx context.Context) (int, error) {
	eventos, err := s.Store.Outbox().NaoDistribuidos(ctx, 200)
	if err != nil {
		return 0, err
	}
	criadas := 0
	for _, ev := range eventos {
		corpo, err := json.Marshal(envelope{Type: ev.Tipo, Timestamp: ev.OcorridoEm, Data: ev.Dados})
		if err != nil {
			return criadas, fmt.Errorf("evento %s: %w", ev.ID, err)
		}
		err = s.Store.WithTx(ctx, func(tx ports.Store) error {
			destinos, err := tx.Destinos().DaConta(ctx, ev.ContaID)
			if err != nil {
				return err
			}
			agora := s.Clock.Now()
			for _, d := range destinos {
				e := domain.Entrega{EventoID: ev.ID, DestinoID: d.ID, ContaID: ev.ContaID, Tipo: ev.Tipo, Corpo: corpo,
					CriadaEm: ev.OcorridoEm, Status: domain.EntregaPendente, ProximaEm: agora, AtualizadaEm: agora}
				if err := tx.Entregas().Criar(ctx, e); err != nil {
					return err
				}
				criadas++
			}
			return tx.Outbox().MarcarDistribuido(ctx, ev.ID)
		})
		if err != nil {
			return criadas, err
		}
	}
	return criadas, nil
}

// ResultadoEntrega resume uma rodada de entregas.
type ResultadoEntrega struct {
	Entregues, Falhas, DLQ int
}

// Entregar envia as entregas vencidas. Sucesso é qualquer 2xx; o resto agenda
// nova tentativa com backoff e jitter, até 24 h depois do evento, e então DLQ.
func (s Saida) Entregar(ctx context.Context) (ResultadoEntrega, error) {
	agora := s.Clock.Now()
	fila, err := s.Store.Entregas().Proximas(ctx, agora, 100)
	if err != nil || len(fila) == 0 {
		return ResultadoEntrega{}, err
	}
	destinos, err := s.Store.Destinos().Listar(ctx)
	if err != nil {
		return ResultadoEntrega{}, err
	}
	porID := make(map[string]domain.Destino, len(destinos))
	for _, d := range destinos {
		porID[d.ID] = d
	}

	paralelas := s.Paralelas
	if paralelas <= 0 {
		paralelas = 8
	}
	var (
		mu  sync.Mutex
		r   ResultadoEntrega
		wg  sync.WaitGroup
		sem = make(chan struct{}, paralelas)
	)
	for _, e := range fila {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			final := s.tentar(ctx, e, porID)
			mu.Lock()
			defer mu.Unlock()
			switch final.Status {
			case domain.EntregaEntregue:
				r.Entregues++
			case domain.EntregaDLQ:
				r.DLQ++
			default:
				r.Falhas++
			}
		}()
	}
	wg.Wait()
	return r, nil
}

func (s Saida) tentar(ctx context.Context, e domain.Entrega, destinos map[string]domain.Destino) domain.Entrega {
	agora := s.Clock.Now()
	e.Tentativas++
	e.AtualizadaEm = agora
	d, ok := destinos[e.DestinoID]
	if !ok {
		e.Status, e.UltimoHTTP, e.UltimoErro = domain.EntregaDLQ, 0, "destino removido"
		_ = s.Store.Entregas().Atualizar(ctx, e)
		return e
	}
	chave, err := s.Vault.Decifrar(ctx, d.SegredoCifrado)
	if err != nil {
		e.Status, e.UltimoErro = domain.EntregaDLQ, "segredo do destino não decifra: master key trocada?"
		_ = s.Store.Entregas().Atualizar(ctx, e)
		return e
	}
	status, err := s.Enviador.Enviar(ctx, d.URL, Assinar(chave, e.EventoID, agora, e.Corpo), e.Corpo)
	e.UltimoHTTP, e.UltimoErro = status, ""
	switch {
	case err != nil:
		e.UltimoErro = err.Error()
	case status >= 200 && status < 300:
		e.Status = domain.EntregaEntregue
		_ = s.Store.Entregas().Atualizar(ctx, e)
		return e
	default:
		e.UltimoErro = "HTTP " + strconv.Itoa(status)
	}
	if agora.Sub(e.CriadaEm) >= domain.ValidadeEntrega {
		e.Status = domain.EntregaDLQ
	} else {
		espera := domain.EsperaAposFalhas(e.Tentativas)
		e.ProximaEm = agora.Add(espera + time.Duration(rand.Int64N(int64(espera)/10+1))) //nolint:gosec // G404: jitter, não segurança
	}
	_ = s.Store.Entregas().Atualizar(ctx, e)
	return e
}

// Assinar monta os cabeçalhos do Standard Webhooks: webhook-signature é
// v1,base64(HMAC-SHA256(chave, "id.timestamp.corpo")).
func Assinar(chave []byte, id string, quando time.Time, corpo []byte) map[string]string {
	ts := strconv.FormatInt(quando.Unix(), 10)
	mac := hmac.New(sha256.New, chave)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(corpo)
	return map[string]string{
		"webhook-id":        id,
		"webhook-timestamp": ts,
		"webhook-signature": "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	}
}

// Reenviar devolve à fila as entregas do evento que estão na DLQ.
func (s Saida) Reenviar(ctx context.Context, eventoID string) (int, error) {
	return s.Store.Entregas().Reenviar(ctx, eventoID, s.Clock.Now())
}

// Entregas lista as entregas no status pedido que o acesso do contexto enxerga
// (chave restrita a uma conta só vê as dela).
func (s Saida) Entregas(ctx context.Context, status domain.StatusEntrega, limite int) ([]domain.Entrega, error) {
	acesso := AcessoDe(ctx)
	busca := limite
	if acesso.ContaID != "" {
		busca = 500 // as de outras contas são descartadas abaixo
	}
	todas, err := s.Store.Entregas().Listar(ctx, status, busca)
	if err != nil {
		return nil, err
	}
	visiveis := make([]domain.Entrega, 0, len(todas))
	for _, e := range todas {
		if acesso.enxerga(e.ContaID) && len(visiveis) < limite {
			visiveis = append(visiveis, e)
		}
	}
	return visiveis, nil
}

// ReenviarVisivel é o Reenviar para quem chega pela API: só reenvia se o
// evento tiver entrega na DLQ que o acesso do contexto enxergue; senão,
// ErrNaoEncontrado.
func (s Saida) ReenviarVisivel(ctx context.Context, eventoID string) (int, error) {
	dlq, err := s.Entregas(ctx, domain.EntregaDLQ, 500)
	if err != nil {
		return 0, err
	}
	if !slices.ContainsFunc(dlq, func(e domain.Entrega) bool { return e.EventoID == eventoID }) {
		return 0, fmt.Errorf("evento %s sem entrega na DLQ: %w", eventoID, ports.ErrNaoEncontrado)
	}
	return s.Reenviar(ctx, eventoID)
}

// DLQ lista as entregas que desistiram.
func (s Saida) DLQ(ctx context.Context, limite int) ([]domain.Entrega, error) {
	return s.Store.Entregas().Listar(ctx, domain.EntregaDLQ, limite)
}
