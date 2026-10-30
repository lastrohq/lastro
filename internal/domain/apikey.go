package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Escopo é uma permissão de API key (RN-60).
type Escopo string

// Escopos conhecidos.
const (
	EscopoCobRead        Escopo = "cob:read"
	EscopoCobWrite       Escopo = "cob:write"
	EscopoCobvRead       Escopo = "cobv:read"
	EscopoCobvWrite      Escopo = "cobv:write"
	EscopoPixRead        Escopo = "pix:read"
	EscopoDevolucaoWrite Escopo = "devolucao:write"
	EscopoRecRead        Escopo = "rec:read"
	EscopoRecWrite       Escopo = "rec:write"
	EscopoCobrRead       Escopo = "cobr:read"
	EscopoCobrWrite      Escopo = "cobr:write"
	EscopoWebhookAdmin   Escopo = "webhook:admin"
	EscopoContaAdmin     Escopo = "conta:admin"
)

// TodosEscopos lista os escopos válidos, em ordem estável.
var TodosEscopos = []Escopo{
	EscopoCobRead, EscopoCobWrite, EscopoCobvRead, EscopoCobvWrite, EscopoPixRead,
	EscopoDevolucaoWrite, EscopoRecRead, EscopoRecWrite, EscopoCobrRead, EscopoCobrWrite, EscopoWebhookAdmin, EscopoContaAdmin,
}

var (
	// ErrAPIKeyInvalida indica dados de criação de API key fora das regras.
	ErrAPIKeyInvalida = errors.New("api key inválida")
	// ErrNaoAutenticado indica token ausente, malformado, desconhecido ou revogado.
	ErrNaoAutenticado = errors.New("não autenticado")
	// ErrTokenWebhook é o ErrNaoAutenticado da rota de webhook, que não usa API key.
	ErrTokenWebhook = fmt.Errorf("%w: token do webhook", ErrNaoAutenticado)
	// ErrSemEscopo indica uma API key válida sem a permissão necessária.
	ErrSemEscopo = errors.New("api key sem o escopo necessário")
)

// APIKey é uma credencial de acesso à API. O segredo nunca é guardado: só o hash.
type APIKey struct {
	ID      string
	Nome    string
	Hash    []byte
	Escopos []Escopo
	// ContaID restringe a chave a uma conta; vazio vale para todas.
	ContaID    string
	CriadaEm   time.Time
	RevogadaEm *time.Time
}

const (
	prefixoToken = "vtm_"
	tamanhoID    = 12
	tamanhoSeg   = 32
)

// NovaAPIKey valida os dados e gera a chave. Devolve o token em claro, que
// só existe neste momento: quem criou precisa guardá-lo.
func NovaAPIKey(nome string, escopos []Escopo, contaID string, agora time.Time) (APIKey, string, error) {
	if nome == "" || len(nome) > 100 {
		return APIKey{}, "", fmt.Errorf("%w: nome deve ter de 1 a 100 caracteres", ErrAPIKeyInvalida)
	}
	if len(escopos) == 0 {
		return APIKey{}, "", fmt.Errorf("%w: informe ao menos um escopo", ErrAPIKeyInvalida)
	}
	vistos := map[Escopo]bool{}
	for _, e := range escopos {
		if !slices.Contains(TodosEscopos, e) {
			return APIKey{}, "", fmt.Errorf("%w: escopo desconhecido %q", ErrAPIKeyInvalida, e)
		}
		vistos[e] = true
	}
	if contaID != "" && vistos[EscopoContaAdmin] {
		return APIKey{}, "", fmt.Errorf("%w: chave restrita a uma conta não pode ter %s", ErrAPIKeyInvalida, EscopoContaAdmin)
	}
	unicos := make([]Escopo, 0, len(vistos))
	for _, e := range TodosEscopos {
		if vistos[e] {
			unicos = append(unicos, e)
		}
	}
	id, segredo := aleatorio(tamanhoID), aleatorio(tamanhoSeg)
	k := APIKey{ID: id, Nome: nome, Hash: hashSegredo(segredo), Escopos: unicos, ContaID: contaID, CriadaEm: agora}
	return k, prefixoToken + id + "_" + segredo, nil
}

// SepararToken extrai id e segredo de "vtm_<id>_<segredo>".
func SepararToken(token string) (id, segredo string, err error) {
	resto, ok := strings.CutPrefix(token, prefixoToken)
	if !ok {
		return "", "", ErrNaoAutenticado
	}
	id, segredo, ok = strings.Cut(resto, "_")
	if !ok || len(id) != tamanhoID || len(segredo) != tamanhoSeg {
		return "", "", ErrNaoAutenticado
	}
	return id, segredo, nil
}

// Confere compara o segredo com o hash em tempo constante e recusa chave revogada.
func (k APIKey) Confere(segredo string) error {
	if subtle.ConstantTimeCompare(hashSegredo(segredo), k.Hash) != 1 || k.RevogadaEm != nil {
		return ErrNaoAutenticado
	}
	return nil
}

// Permite devolve ErrSemEscopo se a chave não tiver o escopo.
func (k APIKey) Permite(e Escopo) error {
	if !slices.Contains(k.Escopos, e) {
		return fmt.Errorf("%w: precisa de %s", ErrSemEscopo, e)
	}
	return nil
}

// hashSegredo usa SHA-256: o segredo tem 190 bits de entropia, então um hash
// lento como bcrypt não acrescenta proteção e só custaria latência por requisição.
func hashSegredo(segredo string) []byte {
	h := sha256.Sum256([]byte(segredo))
	return h[:]
}

const alfabetoToken = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func aleatorio(n int) string {
	const limite = 256 - 256%len(alfabetoToken)
	out := make([]byte, 0, n)
	buf := make([]byte, n*2)
	for len(out) < n {
		_, _ = rand.Read(buf)
		for _, b := range buf {
			if int(b) < limite && len(out) < n {
				out = append(out, alfabetoToken[int(b)%len(alfabetoToken)])
			}
		}
	}
	return string(out)
}
