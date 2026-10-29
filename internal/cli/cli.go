// Package cli implementa os comandos do binário lastro.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"text/tabwriter"
	"time"

	"github.com/lastrohq/lastro/internal/adapters/httpapi"
	"github.com/lastrohq/lastro/internal/adapters/store/sqlite"
	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/platform/vault"
	"github.com/lastrohq/lastro/internal/ports"
)

// Ambiente é o que a CLI recebe do processo; nos testes, tudo é falso.
type Ambiente struct {
	Saida  io.Writer
	Getenv func(string) string
	PSPs   ports.Registry
	Clock  ports.Clock
	Versao string
}

const uso = `uso: lastro <comando>

  serve                          sobe a API (padrão)
  version                        imprime a versão
  chave gerar                    gera uma LASTRO_MASTER_KEY nova
  conta criar  [flags]           cadastra uma conta recebedora
  conta listar                   lista as contas
  conta desativar <id>           impede novas cobranças na conta
  apikey criar --nome N --escopo E [--escopo E...] [--conta ID]
                                 cria uma API key (o token aparece uma vez só)
  apikey listar                  lista as API keys
  apikey revogar <id>            revoga uma API key na hora

escopos: cob:read cob:write cobv:read cobv:write pix:read devolucao:write
         rec:read rec:write webhook:admin conta:admin

variáveis: LASTRO_DB (padrão lastro.db), LASTRO_MASTER_KEY, LASTRO_ADDR (padrão :8080)`

// Run executa o comando em args.
func Run(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		args = []string{"serve"}
	}
	switch args[0] {
	case "version":
		_, err := fmt.Fprintln(amb.Saida, amb.Versao)
		return err
	case "serve":
		return serve(ctx, amb)
	case "chave":
		if len(args) < 2 || args[1] != "gerar" {
			return errors.New("use: lastro chave gerar")
		}
		_, err := fmt.Fprintln(amb.Saida, vault.NovaChave())
		return err
	case "conta":
		return conta(ctx, args[1:], amb)
	case "apikey":
		return apikey(ctx, args[1:], amb)
	case "help", "-h", "--help":
		_, err := fmt.Fprintln(amb.Saida, uso)
		return err
	default:
		return fmt.Errorf("comando desconhecido %q\n\n%s", args[0], uso)
	}
}

func getenv(amb Ambiente, chave, padrao string) string {
	if v := amb.Getenv(chave); v != "" {
		return v
	}
	return padrao
}

// abrir monta o caso de uso de contas a partir do ambiente.
func abrir(ctx context.Context, amb Ambiente) (app.Contas, func(), error) {
	chave := amb.Getenv("LASTRO_MASTER_KEY")
	if chave == "" {
		return app.Contas{}, nil, errors.New("defina LASTRO_MASTER_KEY (gere uma com `lastro chave gerar` e guarde em lugar seguro: sem ela as credenciais não são recuperáveis)")
	}
	v, err := vault.New(chave)
	if err != nil {
		return app.Contas{}, nil, err
	}
	store, err := sqlite.Open(ctx, getenv(amb, "LASTRO_DB", "lastro.db"))
	if err != nil {
		return app.Contas{}, nil, err
	}
	return app.Contas{Store: store, Vault: v, PSPs: amb.PSPs}, func() { _ = store.Close() }, nil
}

func conta(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		return errors.New("use: lastro conta criar|listar|desativar")
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()

	switch args[0] {
	case "criar":
		return contaCriar(ctx, contas, args[1:], amb)
	case "listar":
		lista, err := contas.Listar(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(amb.Saida, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tNOME\tPSP\tCHAVE\tAMBIENTE\tATIVA")
		for _, c := range lista {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%v\n", c.ID, c.Nome, c.PSP, c.ChavePix, c.Ambiente, c.Ativa)
		}
		return tw.Flush()
	case "desativar":
		if len(args) != 2 {
			return errors.New("use: lastro conta desativar <id>")
		}
		if err := contas.Desativar(ctx, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(amb.Saida, "conta %s desativada: novas cobranças serão recusadas\n", args[1])
		return err
	default:
		return fmt.Errorf("subcomando desconhecido %q (use criar, listar ou desativar)", args[0])
	}
}

func contaCriar(ctx context.Context, contas app.Contas, args []string, amb Ambiente) error {
	fs := flag.NewFlagSet("conta criar", flag.ContinueOnError)
	fs.SetOutput(amb.Saida)
	var c domain.Conta
	var ambiente, secretArq, certArq, senhaArq string
	var cred domain.Credenciais
	fs.StringVar(&c.ID, "id", "", "identificador da conta ([a-z0-9-])")
	fs.StringVar(&c.Nome, "nome", "", "nome para exibição")
	fs.StringVar(&c.PSP, "psp", "", "adapter do banco (sandbox, efi, inter...)")
	fs.StringVar(&c.ChavePix, "chave", "", "chave Pix que recebe as cobranças")
	fs.StringVar(&ambiente, "ambiente", string(domain.Homologacao), "homologacao ou producao")
	fs.StringVar(&cred.ClientID, "client-id", "", "client id do OAuth2 do banco")
	// Segredos vêm de arquivo, nunca de flag: flag fica no histórico do shell.
	fs.StringVar(&secretArq, "client-secret-arquivo", "", "arquivo com o client secret")
	fs.StringVar(&certArq, "certificado", "", "arquivo do certificado do mTLS (.p12 ou .pem)")
	fs.StringVar(&senhaArq, "senha-certificado-arquivo", "", "arquivo com a senha do certificado")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c.Ambiente = domain.Ambiente(ambiente)

	var err error
	if cred.ClientSecret, err = lerSegredo(secretArq); err != nil {
		return err
	}
	if cred.SenhaCertificado, err = lerSegredo(senhaArq); err != nil {
		return err
	}
	if certArq != "" {
		if cred.Certificado, err = os.ReadFile(certArq); err != nil { //nolint:gosec // G304: caminho informado pelo próprio operador
			return fmt.Errorf("lendo certificado: %w", err)
		}
	}
	if err := contas.Criar(ctx, c, cred); err != nil {
		return err
	}
	_, err = fmt.Fprintf(amb.Saida, "conta %s criada (%s, %s)\n", c.ID, c.PSP, c.Ambiente)
	return err
}

func lerSegredo(arquivo string) (string, error) {
	if arquivo == "" {
		return "", nil
	}
	b, err := os.ReadFile(arquivo) //nolint:gosec // G304: caminho informado pelo próprio operador
	if err != nil {
		return "", fmt.Errorf("lendo %s: %w", arquivo, err)
	}
	return string(trimFinal(b)), nil
}

// trimFinal tira a quebra de linha que editores e `echo` deixam no fim do arquivo.
func trimFinal(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func serve(ctx context.Context, amb Ambiente) error {
	addr := getenv(amb, "LASTRO_ADDR", ":8080")
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	store := contas.Store
	srv := &http.Server{
		Addr: addr,
		Handler: httpapi.NewRouter(httpapi.Opcoes{
			Versao:       amb.Versao,
			Cobrancas:    app.Cobrancas{Store: store, PSPs: amb.PSPs, Clock: amb.Clock},
			Contas:       contas,
			APIKeys:      app.APIKeys{Store: store, Clock: amb.Clock},
			Idempotencia: app.Idempotencia{Store: store, Clock: amb.Clock},
			Clock:        amb.Clock,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		// ctx já foi cancelado; o desligamento herda os valores dele, mas com prazo próprio.
		fim, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancelar()
		_ = srv.Shutdown(fim)
	}()
	slog.Info("lastro no ar", "addr", addr, "version", amb.Versao)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// escopos é a flag repetível --escopo.
type escopos []domain.Escopo

func (e *escopos) String() string { return fmt.Sprint(*e) }

func (e *escopos) Set(v string) error {
	*e = append(*e, domain.Escopo(v))
	return nil
}

func apikey(ctx context.Context, args []string, amb Ambiente) error {
	if len(args) == 0 {
		return errors.New("use: lastro apikey criar|listar|revogar")
	}
	contas, fechar, err := abrir(ctx, amb)
	if err != nil {
		return err
	}
	defer fechar()
	keys := app.APIKeys{Store: contas.Store, Clock: amb.Clock}

	switch args[0] {
	case "criar":
		fs := flag.NewFlagSet("apikey criar", flag.ContinueOnError)
		fs.SetOutput(amb.Saida)
		var nome, conta string
		var es escopos
		fs.StringVar(&nome, "nome", "", "para que serve a chave (ex.: loja-producao)")
		fs.StringVar(&conta, "conta", "", "restringe a chave a uma conta")
		fs.Var(&es, "escopo", "escopo concedido (repetível)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		k, token, err := keys.Criar(ctx, nome, es, conta)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(amb.Saida, "api key %s criada com %v\n\n  %s\n\nGuarde agora: este token não aparece de novo.\n", k.ID, k.Escopos, token)
		return err
	case "listar":
		lista, err := keys.Listar(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(amb.Saida, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "ID\tNOME\tCONTA\tESCOPOS\tCRIADA\tREVOGADA")
		for _, k := range lista {
			revogada := "-"
			if k.RevogadaEm != nil {
				revogada = k.RevogadaEm.Format(time.DateTime)
			}
			conta := k.ContaID
			if conta == "" {
				conta = "(todas)"
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\t%s\n", k.ID, k.Nome, conta, k.Escopos, k.CriadaEm.Format(time.DateTime), revogada)
		}
		return tw.Flush()
	case "revogar":
		if len(args) != 2 {
			return errors.New("use: lastro apikey revogar <id>")
		}
		if err := keys.Revogar(ctx, args[1]); err != nil {
			return err
		}
		_, err := fmt.Fprintf(amb.Saida, "api key %s revogada\n", args[1])
		return err
	default:
		return fmt.Errorf("subcomando desconhecido %q (use criar, listar ou revogar)", args[0])
	}
}
