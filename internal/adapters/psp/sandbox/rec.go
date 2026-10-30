package sandbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/psp"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

var _ ports.PSPRecorrencia = (*PSP)(nil)

// ispbSandbox é o ISPB fictício que o sandbox usa nos ids de Pix Automático.
const ispbSandbox = "00000000"

const alfanumericos = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func sufixo11() string {
	b := make([]byte, 11)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alfanumericos))))
		b[i] = alfanumericos[n.Int64()]
	}
	return string(b)
}

// CriarRecorrencia registra a recorrência com um idRec no formato da spec:
// R + R/N (retentativa) + ISPB + data + 11 caracteres.
func (p *PSP) CriarRecorrencia(_ context.Context, conta domain.Conta, r domain.Recorrencia) (domain.Recorrencia, error) {
	if r.ValorFixo == ValorRecusado {
		return domain.Recorrencia{}, ErrRecusado
	}
	tentativa := "N"
	if r.PermiteRetentativa() {
		tentativa = "R"
	}
	if r.CriadaEm.IsZero() {
		r.CriadaEm = p.clock.Now()
	}
	r.ID = "R" + tentativa + ispbSandbox + r.CriadaEm.UTC().Format("20060102") + sufixo11()
	r.ContaID, r.Status = conta.ID, domain.RecorrenciaCriada
	p.mu.Lock()
	defer p.mu.Unlock()
	p.recs[r.ID] = r
	return r, nil
}

// ConsultarRecorrencia devolve a recorrência da memória ou, depois de um
// restart, a que o Vintém gravou. Recorrência CRIADA com solicitação há mais
// que o atraso já tem a resposta do pagador: o sandbox a calcula pelo horário
// da solicitação, então ela vale entre processos e depois de reiniciar.
func (p *PSP) ConsultarRecorrencia(ctx context.Context, conta domain.Conta, id string) (domain.Recorrencia, error) {
	p.mu.Lock()
	r, ok := p.recs[id]
	p.mu.Unlock()
	if !ok && p.store != nil {
		var err error
		if r, err = p.store.Recorrencias().Buscar(ctx, id); err == nil {
			ok = true
		} else if !errors.Is(err, ports.ErrNaoEncontrado) {
			return domain.Recorrencia{}, fmt.Errorf("sandbox: lembrando a recorrência %s: %w", id, err)
		}
	}
	if !ok || r.ContaID != conta.ID {
		return domain.Recorrencia{}, fmt.Errorf("sandbox: recorrência %s: %w", id, ports.ErrNaoEncontrado)
	}
	if r.Status == domain.RecorrenciaCriada {
		solicitada, err := p.solicitadaEm(ctx, r.ID)
		if err != nil {
			return domain.Recorrencia{}, err
		}
		if !solicitada.IsZero() && !p.clock.Now().Before(solicitada.Add(p.atraso)) {
			r.Status, r.MotivoRejeicao = desfecho(r)
		}
	}
	return r, nil
}

// desfecho é a resposta simulada do pagador, pelo valor fixo (valores mágicos).
func desfecho(r domain.Recorrencia) (domain.StatusRecorrencia, string) {
	switch r.ValorFixo {
	case ValorRecRejeitada:
		return domain.RecorrenciaRejeitada, "AP13"
	case ValorRecExpirada:
		return domain.RecorrenciaExpirada, ""
	}
	return domain.RecorrenciaAprovada, ""
}

// solicitadaEm é quando a confirmação da recorrência foi pedida pela primeira
// vez: da memória ou, depois de um restart, das solicitações que o Vintém gravou.
func (p *PSP) solicitadaEm(ctx context.Context, idRec string) (time.Time, error) {
	p.mu.Lock()
	t, ok := p.solicitadas[idRec]
	p.mu.Unlock()
	if ok || p.store == nil {
		return t, nil
	}
	sols, err := p.store.Solicitacoes().DaRecorrencia(ctx, idRec)
	if err != nil {
		return time.Time{}, fmt.Errorf("sandbox: lembrando a solicitação de %s: %w", idRec, err)
	}
	if len(sols) == 0 {
		return time.Time{}, nil
	}
	return sols[0].CriadaEm, nil
}

// SolicitarConfirmacao registra a solicitação como ENVIADA ao app do pagador.
func (p *PSP) SolicitarConfirmacao(ctx context.Context, conta domain.Conta, s domain.SolicitacaoRecorrencia) (domain.SolicitacaoRecorrencia, error) {
	rec, err := p.ConsultarRecorrencia(ctx, conta, s.RecorrenciaID)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	if s.CriadaEm.IsZero() {
		s.CriadaEm = p.clock.Now()
	}
	p.mu.Lock()
	p.recs[rec.ID] = rec // depois de um restart, volta para a memória
	if _, ja := p.solicitadas[rec.ID]; !ja {
		p.solicitadas[rec.ID] = s.CriadaEm
	}
	p.mu.Unlock()
	p.avisar(conta, rec)
	s.ID = "SC" + ispbSandbox + s.CriadaEm.UTC().Format("20060102") + sufixo11()
	s.ContaID, s.Status = conta.ID, domain.SolicitacaoEnviada
	return s, nil
}

// Valores mágicos da recorrência, pelo valor fixo, em centavos. O pagador
// "responde" à solicitação depois do atraso, e o sandbox avisa pelo webhook de
// recorrência, como um banco de verdade.
const (
	// ValorRecRejeitada faz o pagador rejeitar (AP13).
	ValorRecRejeitada domain.Money = 77
	// ValorRecExpirada faz a solicitação expirar sem resposta.
	ValorRecExpirada domain.Money = 88
)

// Entregador faz o POST do aviso de recorrência. O padrão usa HTTP; os testes
// trocam por um que guarda o aviso.
type Entregador func(ctx context.Context, url string, corpo []byte) error

// ComEntregador troca quem entrega os avisos de recorrência.
func ComEntregador(e Entregador) Opcao { return func(p *PSP) { p.entregar = e } }

func entregarHTTP(ctx context.Context, url string, corpo []byte) error {
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(corpo))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("sandbox: webhook de recorrência respondeu %d", resp.StatusCode)
	}
	return nil
}

// ConfigurarWebhookRec guarda a URL do aviso de recorrência da conta.
func (p *PSP) ConfigurarWebhookRec(_ context.Context, conta domain.Conta, url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.webhooksRec[conta.ID] = url
	return nil
}

// InterpretarWebhookRec lê o aviso no formato da spec do BCB.
func (p *PSP) InterpretarWebhookRec(conta domain.Conta, corpo []byte) ([]domain.AtualizacaoRecorrencia, error) {
	return psp.WebhookRecBCB(conta, corpo)
}

// avisar entrega, depois do atraso, o aviso da resposta do pagador pelo webhook
// de recorrência, se a conta tiver um neste processo. Sem webhook, a
// conciliação encontra a resposta pelo ConsultarRecorrencia.
func (p *PSP) avisar(conta domain.Conta, r domain.Recorrencia) {
	status, motivo := desfecho(r)
	time.AfterFunc(p.atraso, func() {
		p.mu.Lock()
		url := p.webhooksRec[conta.ID]
		p.mu.Unlock()
		if url == "" {
			return
		}
		item := map[string]any{
			"idRec": r.ID, "status": status,
			"atualizacao": []map[string]string{{"status": string(status), "data": p.clock.Now().UTC().Format(time.RFC3339Nano)}},
		}
		if motivo != "" {
			item["encerramento"] = map[string]any{"rejeicao": map[string]string{"codigo": motivo, "descricao": "Pagador não reconhece o recebedor (sandbox)"}}
		}
		corpo, _ := json.Marshal(map[string]any{"recs": []any{item}})
		if err := p.entregar(context.Background(), url+"/rec", corpo); err != nil {
			slog.Warn("sandbox: aviso de recorrência não entregue", "idRec", r.ID, "erro", err)
		}
	})
}
