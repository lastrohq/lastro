package domain

import (
	"errors"
	"testing"
	"time"
)

func recValida() Recorrencia {
	return Recorrencia{
		Contrato: "63100862", Objeto: "Academia mensal",
		Devedor:     Devedor{Documento: "12345678909", Nome: "Fulano de Tal"},
		DataInicial: NovaData(2026, 11, 10), Periodicidade: Mensal, ValorFixo: 9990, Politica: Retentativa3Em7Dias,
	}
}

func TestRecorrenciaValida(t *testing.T) {
	if err := recValida().Validar(); err != nil {
		t.Fatal(err)
	}
	variavel := recValida()
	variavel.ValorFixo, variavel.PisoValorMaximo = 0, 5000
	if err := variavel.Validar(); err != nil {
		t.Fatalf("valor variável com mínimo: %v", err)
	}
	semFim := recValida()
	semFim.DataFinal = Data{}
	if err := semFim.Validar(); err != nil {
		t.Fatalf("sem data final: %v", err)
	}
}

func TestRecorrenciaInvalida(t *testing.T) {
	casos := map[string]func(*Recorrencia){
		"sem contrato":            func(r *Recorrencia) { r.Contrato = "" },
		"contrato longo":          func(r *Recorrencia) { r.Contrato = "123456789012345678901234567890123456" },
		"objeto longo":            func(r *Recorrencia) { r.Objeto = "123456789012345678901234567890123456" },
		"sem nome do devedor":     func(r *Recorrencia) { r.Devedor.Nome = "" },
		"documento inválido":      func(r *Recorrencia) { r.Devedor.Documento = "12345678900" },
		"sem data inicial":        func(r *Recorrencia) { r.DataInicial = Data{} },
		"fim antes do início":     func(r *Recorrencia) { r.DataFinal = NovaData(2026, 11, 9) },
		"periodicidade inventada": func(r *Recorrencia) { r.Periodicidade = "QUINZENAL" },
		"política inventada":      func(r *Recorrencia) { r.Politica = "PERMITE_SEMPRE" },
		"fixo e piso juntos":      func(r *Recorrencia) { r.PisoValorMaximo = 100 },
		"valor negativo":          func(r *Recorrencia) { r.ValorFixo = -1 },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			r := recValida()
			estraga(&r)
			if err := r.Validar(); !errors.Is(err, ErrRecorrenciaInvalida) {
				t.Fatalf("err = %v; quer ErrRecorrenciaInvalida", err)
			}
		})
	}
}

func TestIDRecorrencia(t *testing.T) {
	for _, id := range []string{"RR1234567820261029abcDEF12345", "RN0000000020261029AAAAAAAAAAA", "RRA1B2C3D420261029x1y2z3w4v5u"} {
		if err := ValidarIDRecorrencia(id); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	for _, id := range []string{"", "RX1234567820261029abcDEF12345", "RR1234567820261029abcDEF1234", "SC1234567820261029abcDEF12345", "rr1234567820261029abcDEF12345"} {
		if err := ValidarIDRecorrencia(id); !errors.Is(err, ErrRecorrenciaInvalida) {
			t.Errorf("%q aceito", id)
		}
	}
}

func TestPermiteRetentativa(t *testing.T) {
	r := recValida()
	if !r.PermiteRetentativa() {
		t.Fatal("PERMITE_3R_7D não permite retentativa")
	}
	r.Politica = SemRetentativa
	if r.PermiteRetentativa() {
		t.Fatal("NAO_PERMITE permite retentativa")
	}
}

func TestTransicoesSolicitacao(t *testing.T) {
	estados := []StatusSolicitacao{SolicitacaoCriada, SolicitacaoEnviada, SolicitacaoRecebida, SolicitacaoAceita, SolicitacaoRejeitada, SolicitacaoExpirada, SolicitacaoCancelada}
	conferirTodas(t, TransicoesSolicitacao, estados,
		map[[2]StatusSolicitacao]bool{
			{SolicitacaoCriada, SolicitacaoEnviada}: true, {SolicitacaoCriada, SolicitacaoExpirada}: true, {SolicitacaoCriada, SolicitacaoCancelada}: true,
			{SolicitacaoEnviada, SolicitacaoRecebida}: true, {SolicitacaoEnviada, SolicitacaoRejeitada}: true,
			{SolicitacaoEnviada, SolicitacaoExpirada}: true, {SolicitacaoEnviada, SolicitacaoCancelada}: true,
			{SolicitacaoRecebida, SolicitacaoAceita}: true, {SolicitacaoRecebida, SolicitacaoRejeitada}: true,
			{SolicitacaoRecebida, SolicitacaoExpirada}: true, {SolicitacaoRecebida, SolicitacaoCancelada}: true,
		},
		map[StatusSolicitacao]bool{SolicitacaoAceita: true, SolicitacaoRejeitada: true, SolicitacaoExpirada: true, SolicitacaoCancelada: true},
	)
}

func TestSolicitacaoValidar(t *testing.T) {
	agora := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	base := SolicitacaoRecorrencia{RecorrenciaID: "RR1234567820261029abcDEF12345", Destinatario: ContaPagador{ISPB: "18236120", Agencia: "0001", Conta: "123456"}, ExpiraEm: agora.Add(7 * 24 * time.Hour)}
	if err := base.Validar(agora); err != nil {
		t.Fatal(err)
	}
	noLimite := base
	noLimite.ExpiraEm = agora.Add(30 * 24 * time.Hour)
	if err := noLimite.Validar(agora); err != nil {
		t.Fatalf("exatamente 30 dias: %v", err)
	}
	casos := map[string]func(*SolicitacaoRecorrencia){
		"sem idRec":        func(s *SolicitacaoRecorrencia) { s.RecorrenciaID = "" },
		"ISPB curto":       func(s *SolicitacaoRecorrencia) { s.Destinatario.ISPB = "1823612" },
		"sem conta":        func(s *SolicitacaoRecorrencia) { s.Destinatario.Conta = "" },
		"agência longa":    func(s *SolicitacaoRecorrencia) { s.Destinatario.Agencia = "00001" },
		"expira agora":     func(s *SolicitacaoRecorrencia) { s.ExpiraEm = agora },
		"expirou":          func(s *SolicitacaoRecorrencia) { s.ExpiraEm = agora.Add(-time.Minute) },
		"passa de 30 dias": func(s *SolicitacaoRecorrencia) { s.ExpiraEm = agora.Add(30*24*time.Hour + time.Second) },
	}
	for nome, estraga := range casos {
		t.Run(nome, func(t *testing.T) {
			s := base
			estraga(&s)
			if err := s.Validar(agora); !errors.Is(err, ErrRecorrenciaInvalida) {
				t.Fatalf("err = %v; quer ErrRecorrenciaInvalida", err)
			}
		})
	}
}

func TestTransitarRecorrenciaESolicitacao(t *testing.T) {
	r := recValida()
	r.Status = RecorrenciaCriada
	if err := r.Transitar(RecorrenciaAprovada); err != nil || r.Status != RecorrenciaAprovada {
		t.Fatalf("CRIADA → APROVADA: %v", err)
	}
	if err := r.Transitar(RecorrenciaRejeitada); !errors.Is(err, ErrTransicaoInvalida) || r.Status != RecorrenciaAprovada {
		t.Fatalf("APROVADA → REJEITADA deveria falhar sem mudar o status: %v, %s", err, r.Status)
	}
	s := SolicitacaoRecorrencia{Status: SolicitacaoRecebida}
	if err := s.Transitar(SolicitacaoAceita); err != nil || s.Status != SolicitacaoAceita {
		t.Fatalf("RECEBIDA → ACEITA: %v", err)
	}
}
