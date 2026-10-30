package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const siteOperar = "../../docs/site/src/content/docs/operar/"

func lerPagina(t *testing.T, nome string) string {
	t.Helper()
	b, err := os.ReadFile(siteOperar + nome) //nolint:gosec // G304: páginas fixas do próprio repositório
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fontesDaCLI junta o código da CLI, sem os testes.
func fontesDaCLI(t *testing.T) string {
	t.Helper()
	arquivos, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var fonte strings.Builder
	for _, a := range arquivos {
		if strings.HasSuffix(a, "_test.go") {
			continue
		}
		b, err := os.ReadFile(a) //nolint:gosec // G304: caminhos do Glob, no próprio pacote
		if err != nil {
			t.Fatal(err)
		}
		fonte.Write(b)
	}
	return fonte.String()
}

// TestTodaVariavelEstaNoSite, TestTodoComandoEstaNoSite e TestTodaFlagEstaNoSite
// reprovam a CLI que ganha algo novo sem documentar.
func TestTodaVariavelEstaNoSite(t *testing.T) {
	pagina := lerPagina(t, "configuracao.md")
	achadas := regexp.MustCompile(`"(VINTEM_[A-Z_]+)"`).FindAllStringSubmatch(fontesDaCLI(t), -1)
	if len(achadas) == 0 {
		t.Fatal("nenhuma variável encontrada no código")
	}
	for _, v := range achadas {
		if !strings.Contains(pagina, "`"+v[1]+"`") {
			t.Errorf("%s fora de configuracao.md", v[1])
		}
	}
}

func TestTodoComandoEstaNoSite(t *testing.T) {
	pagina := lerPagina(t, "cli.md")
	linhas := regexp.MustCompile(`(?m)^  ([a-z]+(?: [a-z]+)?)`).FindAllStringSubmatch(uso, -1)
	if len(linhas) == 0 {
		t.Fatal("nenhum comando encontrado no uso")
	}
	for _, c := range linhas {
		if !strings.Contains(pagina, "`vintem "+c[1]) {
			t.Errorf("comando %q fora de cli.md", c[1])
		}
	}
}

func TestTodaFlagEstaNoSite(t *testing.T) {
	pagina := lerPagina(t, "cli.md")
	achadas := regexp.MustCompile(`fs\.\w+\((?:&[\w.]+, )?"([a-z-]+)"`).FindAllStringSubmatch(fontesDaCLI(t), -1)
	if len(achadas) == 0 {
		t.Fatal("nenhuma flag encontrada no código")
	}
	for _, f := range achadas {
		if !strings.Contains(pagina, "`--"+f[1]+"`") {
			t.Errorf("flag --%s fora de cli.md", f[1])
		}
	}
}
