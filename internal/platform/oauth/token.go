// Package oauth guarda o token OAuth2 de cada conta até perto de expirar, pede
// um só token por vez mesmo com chamadas concorrentes e renova quando o banco
// recusa o token (401). Como pedir o token fica com o adapter de cada banco.
package oauth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
)

// ErrResposta indica uma resposta de token fora do formato da RFC 6749.
var ErrResposta = errors.New("resposta de token inválida")

// Token é um access token com o instante em que expira. O valor nunca aparece
// em log nem em fmt: String e LogValue escondem o segredo.
type Token struct {
	// Valor é o access_token, enviado em Authorization.
	Valor string
	// Tipo é o token_type; vazio vale "Bearer".
	Tipo string
	// ObtidoEm e ExpiraEm marcam a vida do token, para a renovação antecipada.
	ObtidoEm time.Time
	ExpiraEm time.Time
}

// Authorization devolve o valor do header Authorization.
func (t Token) Authorization() string {
	tipo := t.Tipo
	if tipo == "" || strings.EqualFold(tipo, "bearer") {
		tipo = "Bearer"
	}
	return tipo + " " + t.Valor
}

// String esconde o valor do token.
func (t Token) String() string {
	return fmt.Sprintf("oauth.Token{expira em %s}", t.ExpiraEm.Format(time.RFC3339))
}

// GoString esconde o valor do token também em %#v.
func (t Token) GoString() string { return t.String() }

// LogValue esconde o valor do token no slog.
func (t Token) LogValue() slog.Value {
	return slog.GroupValue(slog.Time("expira_em", t.ExpiraEm))
}

// LerResposta interpreta o corpo de sucesso do endpoint de token (RFC 6749,
// seção 5.1). agora é o instante da resposta, de onde conta o expires_in.
func LerResposta(r io.Reader, agora time.Time) (Token, error) {
	var corpo struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   *int64 `json:"expires_in"`
	}
	if err := json.NewDecoder(r).Decode(&corpo); err != nil {
		return Token{}, fmt.Errorf("%w: %w", ErrResposta, err)
	}
	switch {
	case corpo.AccessToken == "":
		return Token{}, fmt.Errorf("%w: access_token ausente", ErrResposta)
	case corpo.ExpiresIn == nil:
		return Token{}, fmt.Errorf("%w: expires_in ausente", ErrResposta)
	case *corpo.ExpiresIn <= 0:
		return Token{}, fmt.Errorf("%w: expires_in deve ser positivo", ErrResposta)
	}
	return Token{
		Valor:    corpo.AccessToken,
		Tipo:     corpo.TokenType,
		ObtidoEm: agora,
		ExpiraEm: agora.Add(time.Duration(*corpo.ExpiresIn) * time.Second),
	}, nil
}
