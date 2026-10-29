package ports

import (
	"context"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
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

// Certificados lê o certificado do mTLS (.p12 com senha, ou PEM com certificado
// e chave) e devolve os dados públicos dele (RN-62). Arquivo ilegível, senha
// errada ou chave que não corresponde ao certificado devolvem um erro que embrulha
// domain.ErrCertificadoInvalido, sem nada do conteúdo na mensagem.
type Certificados interface {
	Ler(arquivo []byte, senha string) (domain.Certificado, error)
}

// Enviador faz o POST de um webhook de saída e devolve o status HTTP. Erro só
// quando nem houve resposta (DNS, conexão, timeout).
type Enviador interface {
	Enviar(ctx context.Context, url string, cabecalhos map[string]string, corpo []byte) (status int, err error)
}
