package oauth_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/platform/oauth"
)

func TestLerResposta(t *testing.T) {
	tok, err := oauth.LerResposta(strings.NewReader(`{"access_token":"abc","token_type":"bearer","expires_in":3600,"scope":"cob.write"}`), inicio)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Valor != "abc" || !tok.ExpiraEm.Equal(inicio.Add(time.Hour)) || !tok.ObtidoEm.Equal(inicio) {
		t.Fatalf("Token = %+v", tok)
	}
	if got := tok.Authorization(); got != "Bearer abc" {
		t.Fatalf("Authorization = %q; quer Bearer abc", got)
	}
}

func TestLerRespostaInvalida(t *testing.T) {
	for nome, corpo := range map[string]string{
		"não é JSON":          `<html>`,
		"sem access_token":    `{"token_type":"Bearer","expires_in":3600}`,
		"sem expires_in":      `{"access_token":"abc"}`,
		"expires_in zero":     `{"access_token":"abc","expires_in":0}`,
		"expires_in negativo": `{"access_token":"abc","expires_in":-1}`,
	} {
		t.Run(nome, func(t *testing.T) {
			if _, err := oauth.LerResposta(strings.NewReader(corpo), inicio); !errors.Is(err, oauth.ErrResposta) {
				t.Fatalf("erro = %v; quer ErrResposta", err)
			}
		})
	}
}

func TestTokenNaoApareceEmLog(t *testing.T) {
	const segredo = "segredo-do-banco-123"
	tok := oauth.Token{Valor: segredo, ExpiraEm: inicio}

	var log bytes.Buffer
	slog.New(slog.NewJSONHandler(&log, nil)).Info("token", "token", tok)
	saidas := map[string]string{
		"%v":   fmt.Sprintf("%v", tok),
		"%+v":  fmt.Sprintf("%+v", tok),
		"%#v":  fmt.Sprintf("%#v", tok),
		"slog": log.String(),
	}
	for formato, saida := range saidas {
		if strings.Contains(saida, segredo) {
			t.Errorf("%s vazou o token: %s", formato, saida)
		}
	}
}
