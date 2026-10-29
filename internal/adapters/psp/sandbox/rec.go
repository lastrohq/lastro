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

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
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
// restart, a que o Lastro gravou.
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
	return r, nil
}

// SolicitarConfirmacao registra a solicitação como ENVIADA ao app do pagador.
func (p *PSP) SolicitarConfirmacao(ctx context.Context, conta domain.Conta, s domain.SolicitacaoRecorrencia) (domain.SolicitacaoRecorrencia, error) {
	rec, err := p.ConsultarRecorrencia(ctx, conta, s.RecorrenciaID)
	if err != nil {
		return domain.SolicitacaoRecorrencia{}, err
	}
	p.mu.Lock()
	p.recs[rec.ID] = rec // depois de um restart, volta para a memória
	p.mu.Unlock()
	p.responder(conta, rec)
	if s.CriadaEm.IsZero() {
		s.CriadaEm = p.clock.Now()
	}
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

// responder simula a resposta do pagador à solicitação, depois do atraso:
// atualiza a recorrência e avisa pelo webhook, se a conta tiver um.
func (p *PSP) responder(conta domain.Conta, r domain.Recorrencia) {
	status, motivo := domain.RecorrenciaAprovada, ""
	switch r.ValorFixo {
	case ValorRecRejeitada:
		status, motivo = domain.RecorrenciaRejeitada, "AP13"
	case ValorRecExpirada:
		status = domain.RecorrenciaExpirada
	}
	time.AfterFunc(p.atraso, func() {
		p.mu.Lock()
		atual := p.recs[r.ID]
		atual.Status, atual.MotivoRejeicao = status, motivo
		p.recs[r.ID] = atual
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
