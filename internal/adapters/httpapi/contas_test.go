package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vintemhq/vintem/internal/adapters/httpapi/gen"
	"github.com/vintemhq/vintem/internal/domain"
)

const contaComCertificado = `{"id":"nova","nome":"Nova","psp":"sandbox","chave":"nova@example.com","ambiente":"homologacao",
	"credenciais":{"clientId":"id","certificado":"UEVNLi4u","senhaCertificado":"senha-que-nao-volta"}}`

func TestContaMostraOsDadosPublicosDoCertificado(t *testing.T) {
	a := novaAPI(t)
	var c gen.Conta
	if st := a.chamar(http.MethodPost, "/vintem/v1/contas", contaComCertificado, nil, &c); st != http.StatusCreated {
		t.Fatalf("criar: %d", st)
	}
	quer := a.certs.Certificado
	if c.Certificado == nil || c.Certificado.Sujeito != quer.Sujeito || c.Certificado.ImpressaoDigital != quer.ImpressaoDigital || !c.Certificado.Validade.Equal(quer.Validade) {
		t.Fatalf("certificado = %+v; quer %+v", c.Certificado, quer)
	}
	if st := a.chamar(http.MethodGet, "/vintem/v1/contas/nova", "", nil, &c); st != http.StatusOK || c.Certificado == nil {
		t.Fatalf("consultar: %d %+v", st, c.Certificado)
	}
	// Conta sem certificado não tem o campo.
	var sem gen.Conta
	if st := a.chamar(http.MethodGet, "/vintem/v1/contas/loja", "", nil, &sem); st != http.StatusOK || sem.Certificado != nil {
		t.Fatalf("conta sem certificado: %d %+v", st, sem.Certificado)
	}
}

func TestContaComCertificadoRecusadoResponde400(t *testing.T) {
	for nome, certs := range map[string]func(a *api){
		"ilegível": func(a *api) { a.certs.Err = fmt.Errorf("%w: senha do .p12 errada", domain.ErrCertificadoInvalido) },
		"vencido":  func(a *api) { a.certs.Certificado.Validade = a.clock.Now().Add(-time.Hour) },
	} {
		t.Run(nome, func(t *testing.T) {
			a := novaAPI(t)
			certs(a)
			rec := a.chamarBruto(http.MethodPost, "/vintem/v1/contas", contaComCertificado, nil)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "certificado") {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), "senha-que-nao-volta") {
				t.Fatalf("a resposta mostrou a senha: %s", rec.Body)
			}
			var p gen.Problema
			if st := a.chamar(http.MethodGet, "/vintem/v1/contas/nova", "", nil, &p); st != http.StatusNotFound {
				t.Fatalf("a conta recusada foi gravada: %d", st)
			}
		})
	}
}

func TestCertificadoDeHomologacaoEmProducaoResponde400(t *testing.T) {
	a := novaAPI(t) // o leitor falso devolve sempre o mesmo certificado
	if st := a.chamar(http.MethodPost, "/vintem/v1/contas", contaComCertificado, nil, nil); st != http.StatusCreated {
		t.Fatalf("homologação: %d", st)
	}
	prod := strings.NewReplacer(`"id":"nova"`, `"id":"nova-prod"`, `nova@example.com`, `prod@example.com`, `"homologacao"`, `"producao"`).Replace(contaComCertificado)
	rec := a.chamarBruto(http.MethodPost, "/vintem/v1/contas", prod, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "homologação nova") {
		t.Fatalf("produção: %d %s", rec.Code, rec.Body)
	}
}
