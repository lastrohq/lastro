package efi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/lastrohq/lastro/internal/adapters/psp"
	"github.com/lastrohq/lastro/internal/domain"
)

// ConfigurarWebhook registra a URL de aviso na chave Pix da conta
// (PUT /v2/webhook/:chave). O Efí acrescenta /pix ao fim da URL, e a rota de
// entrada do Lastro aceita os dois caminhos.
//
// O Efí exige mTLS no servidor que recebe o webhook, com a chave pública dele.
// O Lastro não termina TLS, então pede o skip-mTLS (x-skip-mtls-checking: true).
// Nesse caso o Efí pede outras garantias de que o aviso é dele: a URL do Lastro
// carrega um token secreto por conta, que é o "hmac na URL" da documentação, e
// quem quiser também pode restringir o IP de origem no proxy (ver docs/EFI.md).
func (p *PSP) ConfigurarWebhook(ctx context.Context, conta domain.Conta, endereco string) error {
	corpo := map[string]string{"webhookUrl": endereco}
	return p.chamarCom(ctx, conta, http.MethodPut, "/v2/webhook/"+url.PathEscape(conta.ChavePix), corpo, nil,
		map[string]string{"x-skip-mtls-checking": "true"})
}

// InterpretarWebhook lê o aviso do Efí, que segue o callback da spec do BCB.
//
// Ao cadastrar o webhook, o Efí manda uma notificação de teste, de corpo não
// documentado, e o cadastro falha se a resposta não for 2xx. O token da URL já
// foi conferido antes de chegar aqui, então um corpo sem a chave "pix" é aceito
// sem registrar nada. Um corpo com "pix" fora do formato continua sendo erro.
func (p *PSP) InterpretarWebhook(conta domain.Conta, corpo []byte) ([]domain.PixRecebido, error) {
	var bruto map[string]json.RawMessage
	if err := json.Unmarshal(corpo, &bruto); err == nil {
		if _, temPix := bruto["pix"]; !temPix {
			slog.Info("efi: aviso sem Pix aceito (notificação de teste ou evento não usado)", "conta", conta.ID)
			return nil, nil
		}
	}
	return psp.WebhookPixBCB(conta, corpo)
}
