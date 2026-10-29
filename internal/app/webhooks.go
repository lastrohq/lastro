package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

// Webhooks configura a URL de aviso no banco e recebe os avisos (RN-30).
type Webhooks struct {
	Store        ports.Store
	PSPs         ports.Registry
	Recebimentos Recebimentos
}

// Configurar gera um token novo, registra a URL no PSP e só então grava o
// token: se o PSP recusar, a URL antiga continua valendo.
func (s Webhooks) Configurar(ctx context.Context, contaID, urlPublica string) (string, error) {
	base, err := url.Parse(strings.TrimRight(urlPublica, "/"))
	local := err == nil && (base.Hostname() == "localhost" || base.Hostname() == "127.0.0.1")
	esquemaOK := err == nil && (base.Scheme == "https" || (base.Scheme == "http" && local))
	if err != nil || base.Host == "" || !esquemaOK {
		return "", &ErroValidacao{Violacoes: []Violacao{{Propriedade: "urlPublica", Razao: "use https:// (http só para localhost)", Valor: urlPublica}}}
	}
	conta, err := s.Store.Contas().Buscar(ctx, contaID)
	if err != nil {
		return "", err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapWebhook)
	if err != nil {
		return "", err
	}
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	endereco := base.String() + "/webhooks/pix/" + conta.ID + "/" + token
	if err := psp.ConfigurarWebhook(ctx, conta, endereco); err != nil {
		return "", erroDoPSP(err)
	}
	h := sha256.Sum256([]byte(token))
	if err := s.Store.Contas().SalvarTokenWebhook(ctx, conta.ID, h[:]); err != nil {
		return "", fmt.Errorf("o PSP já recebeu a URL nova, mas o token não foi gravado; configure de novo: %w", err)
	}
	return endereco, nil
}

// Receber confere o token da URL, interpreta o corpo pelo adapter da conta e
// registra os Pix. Só devolve sucesso depois de tudo gravado (RN-30).
func (s Webhooks) Receber(ctx context.Context, contaID, token string, corpo []byte) (int, error) {
	esperado, err := s.Store.Contas().BuscarTokenWebhook(ctx, contaID)
	if errors.Is(err, ports.ErrNaoEncontrado) {
		return 0, domain.ErrNaoAutenticado
	}
	if err != nil {
		return 0, err
	}
	h := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(h[:], esperado) != 1 {
		return 0, domain.ErrNaoAutenticado
	}
	conta, err := s.Store.Contas().Buscar(ctx, contaID)
	if err != nil {
		return 0, err
	}
	psp, err := pspDe(s.PSPs, conta, ports.CapWebhook)
	if err != nil {
		return 0, err
	}
	pixes, err := psp.InterpretarWebhook(conta, corpo)
	if err != nil {
		return 0, err
	}
	for i := range pixes {
		pixes[i].ContaID = conta.ID // o adapter não decide a conta: a URL decide
	}
	return s.Recebimentos.Registrar(ctx, pixes)
}
