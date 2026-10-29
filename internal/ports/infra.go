package ports

import (
	"context"
	"time"
)

// Clock é a fonte de tempo; o domínio nunca chama time.Now() direto.
type Clock interface {
	Now() time.Time
}

// Vault cifra e decifra segredos em repouso: certificados, chaves e client secrets (RN-61).
type Vault interface {
	Cifrar(ctx context.Context, segredo []byte) ([]byte, error)
	Decifrar(ctx context.Context, cifrado []byte) ([]byte, error)
}

// Enviador faz o POST de um webhook de saída e devolve o status HTTP. Erro só
// quando nem houve resposta (DNS, conexão, timeout).
type Enviador interface {
	Enviar(ctx context.Context, url string, cabecalhos map[string]string, corpo []byte) (status int, err error)
}
