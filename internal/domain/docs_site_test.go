package domain_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/vintemhq/vintem/internal/domain"
)

const conceitos = "../../docs/site/src/content/docs/conceitos/"

// TestTodoEventoEstaNoCatalogoDoSite lê as constantes em evento.go: evento novo
// sem linha no catálogo do site reprova aqui.
func TestTodoEventoEstaNoCatalogoDoSite(t *testing.T) {
	fonte, err := os.ReadFile("evento.go")
	if err != nil {
		t.Fatal(err)
	}
	pagina, err := os.ReadFile(conceitos + "webhooks-de-saida.md")
	if err != nil {
		t.Fatal(err)
	}
	eventos := regexp.MustCompile(`TipoEvento = "([^"]+)"`).FindAllStringSubmatch(string(fonte), -1)
	if len(eventos) == 0 {
		t.Fatal("nenhum evento encontrado em evento.go")
	}
	for _, e := range eventos {
		if linha := "| `" + e[1] + "` |"; !strings.Contains(string(pagina), linha) {
			t.Errorf("evento %s sem linha no catálogo de webhooks-de-saida.md", e[1])
		}
	}
}

// TestTodoEscopoEstaNaTabelaDoSite faz o mesmo com os escopos.
func TestTodoEscopoEstaNaTabelaDoSite(t *testing.T) {
	pagina, err := os.ReadFile(conceitos + "api-keys-e-escopos.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range domain.TodosEscopos {
		if linha := "| `" + string(e) + "` |"; !strings.Contains(string(pagina), linha) {
			t.Errorf("escopo %s sem linha na tabela de api-keys-e-escopos.md", e)
		}
	}
}
