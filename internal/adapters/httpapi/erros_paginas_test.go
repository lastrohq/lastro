package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const siteDocs = "../../../docs/site"

// tiposEmitidos lê no código os tipos de erro que a API responde: os literais
// passados a tipoErro e à função p de problemaDe.
func tiposEmitidos(t *testing.T) map[string]bool {
	t.Helper()
	arquivos, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	tipos := map[string]bool{}
	for _, arquivo := range arquivos {
		if strings.HasSuffix(arquivo, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), arquivo, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			chamada, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			funcao, ok := chamada.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			var arg ast.Expr
			switch {
			case funcao.Name == "tipoErro" && len(chamada.Args) == 1:
				arg = chamada.Args[0]
			case funcao.Name == "p" && len(chamada.Args) == 3:
				arg = chamada.Args[1]
			default:
				return true
			}
			if lit, ok := arg.(*ast.BasicLit); ok {
				nome, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatal(err)
				}
				tipos[nome] = true
			} else if id, ok := arg.(*ast.Ident); !ok || id.Name != "tipo" {
				t.Errorf("tipo de erro que não é literal em %s(): este teste não consegue conferir a página", funcao.Name)
			}
			return true
		})
	}
	if len(tipos) == 0 {
		t.Fatal("nenhum tipo de erro encontrado no código")
	}
	return tipos
}

// TestTodoTipoDeErroTemPagina impede a API de ganhar um type que leva a um 404:
// cada tipo precisa de página, de redirect da URL em CamelCase e de uma linha
// na referência. Página de tipo que a API não emite também reprova.
func TestTodoTipoDeErroTemPagina(t *testing.T) {
	tipos := tiposEmitidos(t)

	redirects, err := os.ReadFile(filepath.Join(siteDocs, "public/_redirects"))
	if err != nil {
		t.Fatal(err)
	}
	referencia, err := os.ReadFile(filepath.Join(siteDocs, "src/content/docs/referencia/erros.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tipo := range slices.Sorted(maps.Keys(tipos)) {
		minusculo := strings.ToLower(tipo)
		if _, err := os.Stat(filepath.Join(siteDocs, "src/content/docs/erros", minusculo+".md")); err != nil {
			t.Errorf("%s sem página: %v", tipo, err)
		}
		if linha := "/erros/" + tipo + " /erros/" + minusculo + "/ 301\n"; !strings.Contains(string(redirects), linha) {
			t.Errorf("%s sem a linha %q em public/_redirects", tipo, linha)
		}
		if link := "[`" + tipo + "`](/erros/" + minusculo + "/)"; !strings.Contains(string(referencia), link) {
			t.Errorf("%s sem o link %s em referencia/erros.md", tipo, link)
		}
	}

	paginas, err := os.ReadDir(filepath.Join(siteDocs, "src/content/docs/erros"))
	if err != nil {
		t.Fatal(err)
	}
	emitidos := map[string]bool{}
	for tipo := range tipos {
		emitidos[strings.ToLower(tipo)] = true
	}
	for _, pagina := range paginas {
		if nome := strings.TrimSuffix(pagina.Name(), ".md"); !emitidos[nome] {
			t.Errorf("página %s de um tipo que a API não emite", pagina.Name())
		}
	}
}
