package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/app"
	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports/fake"
)

// leitorPorConteudo devolve um certificado diferente conforme o arquivo, para
// misturar contas boas e ruins no mesmo teste.
type leitorPorConteudo map[string]struct {
	cert domain.Certificado
	err  error
}

func (l leitorPorConteudo) Ler(arquivo []byte, _ string) (domain.Certificado, error) {
	r, ok := l[string(arquivo)]
	if !ok {
		return domain.Certificado{}, fmt.Errorf("%w: arquivo desconhecido no teste", domain.ErrCertificadoInvalido)
	}
	return r.cert, r.err
}

// contaDoV020 grava a conta como o v0.2.0 gravava: credenciais cifradas e
// nenhum dado público do certificado.
func contaDoV020(t *testing.T, svc app.Contas, id string, cred *domain.Credenciais) {
	t.Helper()
	ctx := context.Background()
	if err := svc.Store.Contas().Criar(ctx, domain.Conta{ID: id, Nome: id, PSP: "sandbox", ChavePix: id + "@example.com", Ambiente: domain.Producao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	if cred == nil {
		return
	}
	plano, _ := json.Marshal(cred) //nolint:gosec // G117: serializado só para ser cifrado na linha seguinte, como o app.Contas.Criar
	cifrado, err := svc.Vault.Cifrar(ctx, plano)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Store.Contas().SalvarCredenciais(ctx, id, cifrado); err != nil {
		t.Fatal(err)
	}
}

func TestPreencherCertificadosDasContasAntigas(t *testing.T) {
	svc, s := novoContas(t)
	vencido := domain.Certificado{Sujeito: "CN=velha", ImpressaoDigital: "vv", Validade: agora.Add(-24 * time.Hour)}
	svc.Certificados = leitorPorConteudo{
		"cert-boa":   {cert: certValido},
		"cert-velha": {cert: vencido},
		"cert-ruim":  {err: fmt.Errorf("%w: senha do .p12 errada", domain.ErrCertificadoInvalido)},
	}
	contaDoV020(t, svc, "boa", &domain.Credenciais{ClientID: "a", Certificado: []byte("cert-boa")})
	contaDoV020(t, svc, "velha", &domain.Credenciais{ClientID: "b", Certificado: []byte("cert-velha")})
	contaDoV020(t, svc, "ruim", &domain.Credenciais{ClientID: "c", ClientSecret: "segredo-que-nao-pode-vazar", Certificado: []byte("cert-ruim"), SenhaCertificado: "senha-que-nao-pode-vazar"})
	contaDoV020(t, svc, "sem-cert", &domain.Credenciais{ClientID: "d"})
	contaDoV020(t, svc, "sandbox", nil)

	var log bytes.Buffer
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	ctx := context.Background()
	n, err := svc.PreencherCertificados(ctx)
	if err != nil || n != 2 {
		t.Fatalf("preenchidas = %d, %v; quer 2 (boa e velha)", n, err)
	}
	for id, quer := range map[string]*domain.Certificado{"boa": &certValido, "velha": &vencido, "ruim": nil, "sem-cert": nil, "sandbox": nil} {
		c, _ := s.Contas().Buscar(ctx, id)
		if (quer == nil) != (c.Certificado == nil) || (quer != nil && c.Certificado.ImpressaoDigital != quer.ImpressaoDigital) {
			t.Errorf("%s: certificado = %+v; quer %+v", id, c.Certificado, quer)
		}
	}

	// A conta ruim gera um aviso com o id e o motivo, e nada do conteúdo.
	if !strings.Contains(log.String(), "conta=ruim") || !strings.Contains(log.String(), "senha do .p12 errada") {
		t.Fatalf("o aviso não diz qual conta nem o motivo: %s", log.String())
	}
	for _, segredo := range []string{"segredo-que-nao-pode-vazar", "senha-que-nao-pode-vazar", "cert-ruim"} {
		if strings.Contains(log.String(), segredo) {
			t.Fatalf("o log mostrou %q: %s", segredo, log.String())
		}
	}

	// Rodar de novo não refaz as preenchidas (a ruim é tentada de novo e continua ruim).
	if n, _ := svc.PreencherCertificados(ctx); n != 0 {
		t.Fatalf("segunda rodada preencheu %d; quer 0", n)
	}
}

func TestContaAntigaComCertificadoVencidoRecebeOAviso(t *testing.T) {
	svc, s := novoContas(t)
	vencido := domain.Certificado{Sujeito: "CN=velha", ImpressaoDigital: "vv", Validade: agora.Add(-24 * time.Hour)}
	svc.Certificados = leitorPorConteudo{"cert-velha": {cert: vencido}}
	contaDoV020(t, svc, "velha", &domain.Credenciais{ClientID: "b", Certificado: []byte("cert-velha")})

	ctx := context.Background()
	avisos := app.Certificados{Store: s, Clock: fake.NewClock(agora)}
	if n, _ := avisos.AvisarVencimentos(ctx); n != 0 {
		t.Fatalf("antes de preencher, %d avisos; quer 0 (é o defeito da #79)", n)
	}
	if _, err := svc.PreencherCertificados(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := avisos.AvisarVencimentos(ctx); err != nil || n != 1 {
		t.Fatalf("depois de preencher, %d avisos, %v; quer 1", n, err)
	}
}

func TestPreencherSemLeitorFalha(t *testing.T) {
	svc, _ := novoContas(t)
	svc.Certificados = nil
	if _, err := svc.PreencherCertificados(context.Background()); err == nil {
		t.Fatal("preencheu sem leitor de certificado")
	}
}
