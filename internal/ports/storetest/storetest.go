// Package storetest é a suíte de contrato que todo ports.Store precisa passar:
// o fake em memória hoje, o SQLite e o Postgres depois.
package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports"
)

var base = time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

func cob(conta, txid string, minuto int) domain.Cobranca {
	return domain.Cobranca{
		Txid: domain.Txid(txid), ContaID: conta, Chave: conta + "@example.com", Status: domain.CobrancaAtiva, Valor: 1050,
		Expiracao: domain.ExpiracaoPadrao, CriadaEm: base.Add(time.Duration(minuto) * time.Minute),
		Devedor: &domain.Devedor{Documento: "12345678909", Nome: "Fulano"},
	}
}

func cobv(conta, txid string, minuto int) domain.CobrancaVencimento {
	venc := domain.NovaData(2026, 11, 30)
	return domain.CobrancaVencimento{
		Txid: domain.Txid(txid), ContaID: conta, Chave: conta + "@example.com", Status: domain.CobrancaAtiva, Valor: 15000,
		Multa:      &domain.ValorModalidade{Modalidade: 2, ValorPerc: 200},
		Juros:      &domain.ValorModalidade{Modalidade: 2, ValorPerc: 33},
		Desconto:   &domain.Desconto{Modalidade: 1, DatasFixas: []domain.DescontoData{{Data: venc.SomarDias(-10), ValorPerc: 1000}, {Data: venc.SomarDias(-5), ValorPerc: 500}}},
		Vencimento: venc, ValidadeAposVencimento: 5,
		Devedor:  domain.DevedorCompleto{Devedor: domain.Devedor{Documento: "11222333000181", Nome: "Empresa"}, Email: "fin@example.com", Cidade: "Recife", UF: "PE", CEP: "50000000"},
		CriadaEm: base.Add(time.Duration(minuto) * time.Minute),
	}
}

// comContas cria um Store novo já com as contas c1 e c2, porque cobrança
// pertence a uma conta existente.
func comContas(t *testing.T, novo func(t *testing.T) ports.Store) ports.Store {
	t.Helper()
	s := novo(t)
	for _, id := range []string{"c1", "c2"} {
		c := domain.Conta{ID: id, Nome: "Loja " + id, PSP: "sandbox", ChavePix: id + "@example.com", Ambiente: domain.Homologacao, Ativa: true}
		if err := s.Contas().Criar(context.Background(), c); err != nil {
			t.Fatalf("criando conta %s: %v", id, err)
		}
	}
	return s
}

// Run executa a suíte. novo deve devolver um Store vazio a cada chamada.
func Run(t *testing.T, novo func(t *testing.T) ports.Store) {
	t.Run("conta: cria, busca e recusa duplicada", func(t *testing.T) {
		s, ctx := novo(t), context.Background()
		c := domain.Conta{ID: "c1", Nome: "Loja", PSP: "sandbox", ChavePix: "x@example.com", Ambiente: domain.Homologacao, Ativa: true}
		if err := s.Contas().Criar(ctx, c); err != nil {
			t.Fatal(err)
		}
		if err := s.Contas().Criar(ctx, c); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("duplicada: err = %v", err)
		}
		got, err := s.Contas().Buscar(ctx, "c1")
		if err != nil || got != c {
			t.Fatalf("Buscar = %+v, %v", got, err)
		}
		if _, err := s.Contas().Buscar(ctx, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: err = %v", err)
		}
		lista, err := s.Contas().Listar(ctx)
		if err != nil || len(lista) != 1 {
			t.Fatalf("Listar = %v, %v", lista, err)
		}
	})

	t.Run("conta: atualiza campos editáveis, mas não troca o PSP", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		c, _ := s.Contas().Buscar(ctx, "c1")
		c.Nome, c.Ativa, c.Ambiente, c.PSP = "Outro nome", false, domain.Producao, "efi"
		if err := s.Contas().Atualizar(ctx, c); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Contas().Buscar(ctx, "c1")
		if got.Nome != "Outro nome" || got.Ativa || got.Ambiente != domain.Producao || got.PSP != "sandbox" {
			t.Fatalf("depois de atualizar: %+v", got)
		}
		if err := s.Contas().Atualizar(ctx, domain.Conta{ID: "nada", Ambiente: domain.Producao}); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: err = %v", err)
		}
	})

	t.Run("conta: credenciais cifradas vão e voltam intactas", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		if _, err := s.Contas().BuscarCredenciais(ctx, "c1"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("sem credenciais: err = %v", err)
		}
		blob := []byte{1, 0, 255, 42, 7}
		if err := s.Contas().SalvarCredenciais(ctx, "c1", blob); err != nil {
			t.Fatal(err)
		}
		got, err := s.Contas().BuscarCredenciais(ctx, "c1")
		if err != nil || string(got) != string(blob) {
			t.Fatalf("BuscarCredenciais = %v, %v", got, err)
		}
		if err := s.Contas().SalvarCredenciais(ctx, "nada", blob); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("conta inexistente: err = %v", err)
		}
	})

	t.Run("conta: chave Pix é única e encontra a conta", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		got, err := s.Contas().BuscarPorChave(ctx, "c2@example.com")
		if err != nil || got.ID != "c2" {
			t.Fatalf("BuscarPorChave = %+v, %v", got, err)
		}
		if _, err := s.Contas().BuscarPorChave(ctx, "ninguem@example.com"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("chave sem dono: err = %v", err)
		}
		dup := domain.Conta{ID: "c3", Nome: "X", PSP: "sandbox", ChavePix: "c1@example.com", Ambiente: domain.Homologacao, Ativa: true}
		if err := s.Contas().Criar(ctx, dup); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("chave repetida: err = %v", err)
		}
	})

	t.Run("cobrança: txid é único na instância", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		if err := s.Cobrancas().Criar(ctx, cob("c1", "txid00000000000000000000001", 0)); err != nil {
			t.Fatal(err)
		}
		if err := s.Cobrancas().Criar(ctx, cob("c1", "txid00000000000000000000001", 1)); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("mesma conta: err = %v", err)
		}
		if err := s.Cobrancas().Criar(ctx, cob("c2", "txid00000000000000000000001", 1)); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("outra conta: err = %v", err)
		}
		got, err := s.Cobrancas().BuscarPorTxid(ctx, "txid00000000000000000000001")
		if err != nil || got.ContaID != "c1" {
			t.Fatalf("BuscarPorTxid = %+v, %v", got, err)
		}
		if _, err := s.Cobrancas().BuscarPorTxid(ctx, "txidinexistente000000000001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: err = %v", err)
		}
	})

	t.Run("cobrança: busca devolve o que foi gravado", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		c := cob("c1", "txid00000000000000000000001", 0)
		_ = s.Cobrancas().Criar(ctx, c)
		got, err := s.Cobrancas().Buscar(ctx, "c1", c.Txid)
		if err != nil {
			t.Fatal(err)
		}
		if got.Chave != c.Chave || got.Valor != c.Valor || got.Status != c.Status || !got.CriadaEm.Equal(c.CriadaEm) || got.Devedor == nil || *got.Devedor != *c.Devedor || got.Revisao != 0 {
			t.Fatalf("Buscar = %+v", got)
		}
		if _, err := s.Cobrancas().Buscar(ctx, "c2", c.Txid); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("outra conta: err = %v", err)
		}
	})

	t.Run("cobrança: atualização com revisão desatualizada é recusada", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		_ = s.Cobrancas().Criar(ctx, cob("c1", "txid00000000000000000000001", 0))
		a, _ := s.Cobrancas().Buscar(ctx, "c1", "txid00000000000000000000001")
		b := a

		a.Status = domain.CobrancaConcluida
		if err := s.Cobrancas().Atualizar(ctx, &a); err != nil || a.Revisao != 1 {
			t.Fatalf("primeira escrita: err=%v revisao=%d", err, a.Revisao)
		}
		b.Status = domain.CobrancaRemovidaPeloPSP
		if err := s.Cobrancas().Atualizar(ctx, &b); !errors.Is(err, ports.ErrConflito) {
			t.Fatalf("escrita concorrente: err = %v", err)
		}
		got, _ := s.Cobrancas().Buscar(ctx, "c1", a.Txid)
		if got.Status != domain.CobrancaConcluida {
			t.Fatalf("a escrita perdedora venceu: %s", got.Status)
		}
	})

	t.Run("cobrança: listagem filtra e pagina sem repetir", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		for i, tx := range []string{"a", "b", "c", "d", "e"} {
			c := cob("c1", "txid0000000000000000000000"+tx, i)
			if tx == "c" {
				c.Status = domain.CobrancaConcluida
			}
			_ = s.Cobrancas().Criar(ctx, c)
		}
		_ = s.Cobrancas().Criar(ctx, cob("c2", "txid0000000000000000000000z", 9))

		var vistos []domain.Txid
		cursor := ""
		for range 10 {
			pag, prox, err := s.Cobrancas().Listar(ctx, ports.FiltroCobrancas{ContaID: "c1", Status: domain.CobrancaAtiva, Limite: 2, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range pag {
				vistos = append(vistos, c.Txid)
			}
			if prox == "" {
				break
			}
			cursor = prox
		}
		quer := []domain.Txid{"txid0000000000000000000000a", "txid0000000000000000000000b", "txid0000000000000000000000d", "txid0000000000000000000000e"}
		if len(vistos) != len(quer) {
			t.Fatalf("listou %v, quer %v", vistos, quer)
		}
		for i := range quer {
			if vistos[i] != quer[i] {
				t.Fatalf("listou %v, quer %v", vistos, quer)
			}
		}

		pag, _, _ := s.Cobrancas().Listar(ctx, ports.FiltroCobrancas{Janela: ports.Janela{Inicio: base.Add(time.Minute), Fim: base.Add(3 * time.Minute)}})
		if len(pag) != 2 {
			t.Fatalf("janela [1min, 3min) devolveu %d cobranças, quer 2", len(pag))
		}

		filtro := ports.FiltroCobrancas{ContaID: "c1", Status: domain.CobrancaAtiva}
		if n, err := s.Cobrancas().Contar(ctx, filtro); err != nil || n != 4 {
			t.Fatalf("Contar = %d, %v", n, err)
		}
		filtro.Pular, filtro.Limite = 2, 10
		pag, _, _ = s.Cobrancas().Listar(ctx, filtro)
		if len(pag) != 2 || pag[0].Txid != "txid0000000000000000000000d" {
			t.Fatalf("Pular 2 devolveu %+v", pag)
		}
		if n, _ := s.Cobrancas().Contar(ctx, ports.FiltroCobrancas{Documento: "12345678909"}); n != 6 {
			t.Fatalf("filtro por documento contou %d, quer 6", n)
		}
		if n, _ := s.Cobrancas().Contar(ctx, ports.FiltroCobrancas{Documento: "00000000000"}); n != 0 {
			t.Fatalf("documento sem cobrança contou %d", n)
		}
	})

	t.Run("api key: cria, busca, lista e revoga", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		geral := domain.APIKey{ID: "aaaaaaaaaaaa", Nome: "geral", Hash: []byte{1, 2, 3}, Escopos: []domain.Escopo{domain.EscopoCobRead, domain.EscopoCobWrite}, CriadaEm: base}
		presa := domain.APIKey{ID: "bbbbbbbbbbbb", Nome: "presa", Hash: []byte{4}, Escopos: []domain.Escopo{domain.EscopoCobRead}, ContaID: "c1", CriadaEm: base.Add(time.Minute)}
		for _, k := range []domain.APIKey{geral, presa} {
			if err := s.APIKeys().Criar(ctx, k); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.APIKeys().Criar(ctx, geral); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("duplicada: err = %v", err)
		}
		got, err := s.APIKeys().Buscar(ctx, "bbbbbbbbbbbb")
		if err != nil || got.ContaID != "c1" || string(got.Hash) != string([]byte{4}) || len(got.Escopos) != 1 || !got.CriadaEm.Equal(presa.CriadaEm) || got.RevogadaEm != nil {
			t.Fatalf("Buscar = %+v, %v", got, err)
		}
		if lista, _ := s.APIKeys().Listar(ctx); len(lista) != 2 || lista[0].ID != geral.ID {
			t.Fatalf("Listar = %+v", lista)
		}
		quando := base.Add(time.Hour)
		if err := s.APIKeys().Revogar(ctx, geral.ID, quando); err != nil {
			t.Fatal(err)
		}
		_ = s.APIKeys().Revogar(ctx, geral.ID, quando.Add(time.Hour))
		got, _ = s.APIKeys().Buscar(ctx, geral.ID)
		if got.RevogadaEm == nil || !got.RevogadaEm.Equal(quando) {
			t.Fatalf("revogação: %v", got.RevogadaEm)
		}
		if err := s.APIKeys().Revogar(ctx, "nada", quando); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("revogar inexistente: err = %v", err)
		}
		if _, err := s.APIKeys().Buscar(ctx, "nada"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("buscar inexistente: err = %v", err)
		}
	})

	t.Run("idempotência: grava, substitui, isola por dono e remove", func(t *testing.T) {
		s, ctx := novo(t), context.Background()
		reg := domain.RegistroIdempotencia{Dono: "k1", Chave: "pedido-1", Hash: []byte{9, 9}, CriadaEm: base}
		if _, err := s.Idempotencia().Buscar(ctx, "k1", "pedido-1"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("vazio: err = %v", err)
		}
		if err := s.Idempotencia().Salvar(ctx, reg); err != nil {
			t.Fatal(err)
		}
		reg.Status, reg.ContentType, reg.Corpo = 201, "application/json", []byte(`{"ok":true}`)
		if err := s.Idempotencia().Salvar(ctx, reg); err != nil {
			t.Fatal(err)
		}
		got, err := s.Idempotencia().Buscar(ctx, "k1", "pedido-1")
		if err != nil || got.Status != 201 || string(got.Corpo) != `{"ok":true}` || got.ContentType != "application/json" || string(got.Hash) != string(reg.Hash) || !got.CriadaEm.Equal(base) {
			t.Fatalf("Buscar = %+v, %v", got, err)
		}
		if _, err := s.Idempotencia().Buscar(ctx, "k2", "pedido-1"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatal("chave de outro dono colidiu")
		}
		if err := s.Idempotencia().Remover(ctx, "k1", "pedido-1"); err != nil {
			t.Fatal(err)
		}
		if err := s.Idempotencia().Remover(ctx, "k1", "pedido-1"); err != nil {
			t.Fatalf("remover de novo: %v", err)
		}
		if _, err := s.Idempotencia().Buscar(ctx, "k1", "pedido-1"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatal("não removeu")
		}
	})

	t.Run("cobv: grava tudo, compartilha o txid com a cob e pagina", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		v := cobv("c1", "cobv00000000000000000000001", 0)
		if err := s.CobrancasV().Criar(ctx, v); err != nil {
			t.Fatal(err)
		}
		got, err := s.CobrancasV().BuscarPorTxid(ctx, v.Txid)
		if err != nil {
			t.Fatal(err)
		}
		if got.Vencimento != v.Vencimento || got.ValidadeAposVencimento != 5 || got.Multa == nil || *got.Multa != *v.Multa ||
			got.Juros == nil || *got.Juros != *v.Juros || got.Desconto == nil || len(got.Desconto.DatasFixas) != 2 ||
			got.Desconto.DatasFixas[1] != v.Desconto.DatasFixas[1] || got.Abatimento != nil ||
			got.Devedor != v.Devedor || got.Chave != v.Chave || !got.CriadaEm.Equal(v.CriadaEm) || got.Revisao != 0 {
			t.Fatalf("BuscarPorTxid = %+v", got)
		}
		if err := s.Cobrancas().Criar(ctx, cob("c1", string(v.Txid), 1)); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("cob com o txid de uma cobv: err = %v", err)
		}
		_ = s.Cobrancas().Criar(ctx, cob("c1", "cob000000000000000000000001", 1))
		if err := s.CobrancasV().Criar(ctx, cobv("c2", "cob000000000000000000000001", 1)); !errors.Is(err, ports.ErrJaExiste) {
			t.Fatalf("cobv com o txid de uma cob: err = %v", err)
		}

		got.Status = domain.CobrancaConcluida
		stale := got
		if err := s.CobrancasV().Atualizar(ctx, &got); err != nil || got.Revisao != 1 {
			t.Fatalf("Atualizar: %v rev %d", err, got.Revisao)
		}
		if err := s.CobrancasV().Atualizar(ctx, &stale); !errors.Is(err, ports.ErrConflito) {
			t.Fatalf("revisão velha: err = %v", err)
		}

		for i := 1; i <= 3; i++ {
			_ = s.CobrancasV().Criar(ctx, cobv("c2", "cobv0000000000000000000000"+string(rune('a'+i)), i))
		}
		pag, _, err := s.CobrancasV().Listar(ctx, ports.FiltroCobrancas{ContaID: "c2", Limite: 2, Pular: 1})
		if err != nil || len(pag) != 2 || pag[0].Txid != "cobv0000000000000000000000c" {
			t.Fatalf("Listar: %+v, %v", pag, err)
		}
		if n, _ := s.CobrancasV().Contar(ctx, ports.FiltroCobrancas{Status: domain.CobrancaAtiva}); n != 3 {
			t.Fatalf("Contar ativas = %d, quer 3", n)
		}
		if _, err := s.CobrancasV().BuscarPorTxid(ctx, "nadanadanadanadanadanadana1"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatalf("inexistente: err = %v", err)
		}
	})

	t.Run("transação: erro desfaz tudo, sucesso grava tudo", func(t *testing.T) {
		s, ctx := comContas(t, novo), context.Background()
		falha := errors.New("falhou no meio")
		err := s.WithTx(ctx, func(tx ports.Store) error {
			_ = tx.Cobrancas().Criar(ctx, cob("c1", "txid00000000000000000000001", 0))
			_ = tx.Outbox().Adicionar(ctx, domain.Evento{ID: "e1", Tipo: domain.EventoCobrancaCriada})
			return falha
		})
		if !errors.Is(err, falha) {
			t.Fatalf("WithTx err = %v", err)
		}
		if _, err := s.Cobrancas().Buscar(ctx, "c1", "txid00000000000000000000001"); !errors.Is(err, ports.ErrNaoEncontrado) {
			t.Fatal("rollback não desfez a cobrança")
		}

		err = s.WithTx(ctx, func(tx ports.Store) error {
			if err := tx.Cobrancas().Criar(ctx, cob("c1", "txid00000000000000000000002", 0)); err != nil {
				return err
			}
			return tx.Outbox().Adicionar(ctx, domain.Evento{ID: "e2", Tipo: domain.EventoCobrancaCriada})
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Cobrancas().Buscar(ctx, "c1", "txid00000000000000000000002"); err != nil {
			t.Fatalf("commit não gravou: %v", err)
		}
	})
}
