package domain

import (
	"errors"
	"testing"
)

// conferirTodas testa todos os pares de estados: o que está em validas passa,
// todo o resto é recusado com ErrTransicaoInvalida.
func conferirTodas[S ~string](t *testing.T, tr Transicoes[S], estados []S, validas map[[2]S]bool, terminais map[S]bool) {
	t.Helper()
	for _, de := range estados {
		if got := tr.Terminal(de); got != terminais[de] {
			t.Errorf("Terminal(%s) = %v, quer %v", de, got, terminais[de])
		}
		for _, para := range estados {
			err := tr.Validar(de, para)
			if validas[[2]S{de, para}] {
				if err != nil {
					t.Errorf("%s → %s deveria passar: %v", de, para, err)
				}
			} else if !errors.Is(err, ErrTransicaoInvalida) {
				t.Errorf("%s → %s deveria ser recusada, err = %v", de, para, err)
			}
		}
	}
}

func TestTransicoesCobranca(t *testing.T) {
	estados := []StatusCobranca{CobrancaAtiva, CobrancaConcluida, CobrancaRemovidaPeloUsuarioRecebedor, CobrancaRemovidaPeloPSP}
	conferirTodas(t, TransicoesCobranca, estados,
		map[[2]StatusCobranca]bool{
			{CobrancaAtiva, CobrancaConcluida}:                    true,
			{CobrancaAtiva, CobrancaRemovidaPeloUsuarioRecebedor}: true,
			{CobrancaAtiva, CobrancaRemovidaPeloPSP}:              true,
		},
		map[StatusCobranca]bool{CobrancaConcluida: true, CobrancaRemovidaPeloUsuarioRecebedor: true, CobrancaRemovidaPeloPSP: true},
	)
}

func TestTransicoesDevolucao(t *testing.T) {
	estados := []StatusDevolucao{DevolucaoEmProcessamento, DevolucaoDevolvido, DevolucaoNaoRealizado}
	conferirTodas(t, TransicoesDevolucao, estados,
		map[[2]StatusDevolucao]bool{
			{DevolucaoEmProcessamento, DevolucaoDevolvido}:    true,
			{DevolucaoEmProcessamento, DevolucaoNaoRealizado}: true,
		},
		map[StatusDevolucao]bool{DevolucaoDevolvido: true, DevolucaoNaoRealizado: true},
	)
}

func TestTransicoesRecorrencia(t *testing.T) {
	estados := []StatusRecorrencia{RecorrenciaCriada, RecorrenciaAprovada, RecorrenciaRejeitada, RecorrenciaExpirada, RecorrenciaCancelada}
	conferirTodas(t, TransicoesRecorrencia, estados,
		map[[2]StatusRecorrencia]bool{
			{RecorrenciaCriada, RecorrenciaAprovada}:    true,
			{RecorrenciaCriada, RecorrenciaRejeitada}:   true,
			{RecorrenciaCriada, RecorrenciaExpirada}:    true,
			{RecorrenciaAprovada, RecorrenciaCancelada}: true,
			{RecorrenciaAprovada, RecorrenciaExpirada}:  true,
		},
		map[StatusRecorrencia]bool{RecorrenciaRejeitada: true, RecorrenciaExpirada: true, RecorrenciaCancelada: true},
	)
}

func TestTransicoesCobrancaRecorrente(t *testing.T) {
	estados := []StatusCobrancaRecorrente{CobrCriada, CobrAtiva, CobrConcluida, CobrExpirada, CobrRejeitada, CobrCancelada}
	conferirTodas(t, TransicoesCobrancaRecorrente, estados,
		map[[2]StatusCobrancaRecorrente]bool{
			{CobrCriada, CobrAtiva}:     true,
			{CobrCriada, CobrRejeitada}: true,
			{CobrCriada, CobrCancelada}: true,
			{CobrAtiva, CobrConcluida}:  true,
			{CobrAtiva, CobrExpirada}:   true,
			{CobrAtiva, CobrRejeitada}:  true,
			{CobrAtiva, CobrCancelada}:  true,
		},
		map[StatusCobrancaRecorrente]bool{CobrConcluida: true, CobrExpirada: true, CobrRejeitada: true, CobrCancelada: true},
	)
}

func TestEstadoDesconhecidoNaoTransita(t *testing.T) {
	if err := TransicoesCobranca.Validar("INVENTADO", CobrancaConcluida); !errors.Is(err, ErrTransicaoInvalida) {
		t.Fatalf("estado desconhecido transitou: %v", err)
	}
}
