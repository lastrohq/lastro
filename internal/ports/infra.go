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
