package domain

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// Destino é para onde o Vintém envia os eventos (RN-32): uma URL da aplicação
// do cliente, para uma conta ou, com ContaID vazio, para todas.
type Destino struct {
	ID      string
	ContaID string
	URL     string
	// SegredoCifrado é a chave HMAC cifrada pelo Vault; em claro, só na hora de assinar.
	SegredoCifrado []byte
	CriadoEm       time.Time
}

// NovoSegredoWebhook gera a chave HMAC e a devolve também no formato do
// Standard Webhooks (whsec_ + base64), que é o que o cliente configura.
func NovoSegredoWebhook() (chave []byte, whsec string) {
	chave = make([]byte, 32)
	_, _ = rand.Read(chave)
	return chave, "whsec_" + base64.StdEncoding.EncodeToString(chave)
}

// StatusEntrega é o estado de uma entrega de evento a um destino.
type StatusEntrega string

// Estados de entrega.
const (
	EntregaPendente StatusEntrega = "pendente"
	EntregaEntregue StatusEntrega = "entregue"
	// EntregaDLQ é a fila de mortos: falhou por 24 h e espera reenvio manual.
	EntregaDLQ StatusEntrega = "dlq"
)

// ValidadeEntrega é por quanto tempo o Vintém insiste antes da DLQ (RN-32).
const ValidadeEntrega = 24 * time.Hour

// Entrega é um evento a caminho de um destino. Corpo é montado uma vez, na
// distribuição, e repetido byte a byte em cada tentativa.
type Entrega struct {
	EventoID  string
	DestinoID string
	ContaID   string
	Tipo      TipoEvento
	Corpo     []byte
	// CriadaEm é quando o evento aconteceu: a validade de 24 h conta daqui.
	CriadaEm     time.Time
	Status       StatusEntrega
	Tentativas   int
	ProximaEm    time.Time
	UltimoHTTP   int
	UltimoErro   string
	AtualizadaEm time.Time
}

// esperas é o intervalo antes da tentativa n+1, depois de n falhas.
var esperas = []time.Duration{
	5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute,
	30 * time.Minute, time.Hour, 2 * time.Hour, 4 * time.Hour,
}

// EsperaAposFalhas devolve quanto esperar depois de n falhas: cresce até 4 h
// e para aí. O jitter fica com quem agenda.
func EsperaAposFalhas(n int) time.Duration {
	if n < 1 {
		return 0
	}
	if n > len(esperas) {
		return esperas[len(esperas)-1]
	}
	return esperas[n-1]
}
