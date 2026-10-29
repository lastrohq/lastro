package oauth

import (
	"io"
	"net/http"
)

// Transport põe o token da conta em cada requisição. Se o banco responder 401,
// invalida o token e tenta mais uma vez com um token novo.
type Transport struct {
	// Base faz a requisição de fato; nil usa http.DefaultTransport. Para mTLS,
	// passe o transporte com o certificado da conta.
	Base   http.RoundTripper
	Cache  *Cache
	Chave  string
	Buscar Buscar
}

// RoundTrip implementa http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.Cache.Token(req.Context(), t.Chave, t.Buscar)
	if err != nil {
		fecharCorpo(req)
		return nil, err
	}
	resp, err := t.base().RoundTrip(comToken(req, token))
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	// Sem como reenviar o corpo, a tentativa extra não é possível.
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return resp, nil
	}
	t.Cache.Invalidar(t.Chave, token)
	novo, err := t.Cache.Token(req.Context(), t.Chave, t.Buscar)
	if err != nil {
		return resp, nil // fica a resposta 401 original, que explica mais que o erro da busca
	}
	repetida := req.Clone(req.Context())
	if req.GetBody != nil {
		if repetida.Body, err = req.GetBody(); err != nil {
			return resp, nil
		}
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return t.base().RoundTrip(comToken(repetida, novo))
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

// comToken copia a requisição com o header Authorization, sem alterar a original
// (regra de http.RoundTripper).
func comToken(req *http.Request, token Token) *http.Request {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", token.Authorization())
	return r
}

// fecharCorpo cumpre a regra de http.RoundTripper de fechar o corpo mesmo em erro.
func fecharCorpo(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}
