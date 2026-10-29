package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lastrohq/lastro/internal/app"
	"github.com/lastrohq/lastro/internal/domain"
	"github.com/lastrohq/lastro/internal/ports/fake"
)

var vence = time.Date(2027, 1, 31, 12, 0, 0, 0, time.UTC)

func comCertificado(t *testing.T, s *fake.Store, id string, validade time.Time, ativa bool) {
	t.Helper()
	c := domain.Conta{ID: id, Nome: "Loja", PSP: "sandbox", ChavePix: id + "@example.com", Ambiente: domain.Producao, Ativa: ativa,
		Certificado: &domain.Certificado{Sujeito: "CN=" + id, ImpressaoDigital: "fp-" + id, Validade: validade}}
	if err := s.Contas().Criar(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func eventosDeCertificado(t *testing.T, s *fake.Store) []app.DadosCertificado {
	t.Helper()
	evs, err := s.Outbox().NaoDistribuidos(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	var out []app.DadosCertificado
	for _, e := range evs {
		if e.Tipo != domain.EventoCertificadoVencendo {
			continue
		}
		var d app.DadosCertificado
		if err := json.Unmarshal(e.Dados, &d); err != nil {
			t.Fatal(err)
		}
		out = append(out, d)
	}
	return out
}

func TestAvisaTrintaDiasAntesUmaVezSo(t *testing.T) {
	s := fake.NewStore()
	relogio := fake.NewClock(vence.Add(-31 * 24 * time.Hour))
	svc := app.Certificados{Store: s, Clock: relogio}
	comCertificado(t, s, "loja", vence, true)
	ctx := context.Background()

	if n, err := svc.AvisarVencimentos(ctx); err != nil || n != 0 {
		t.Fatalf("a 31 dias: %d avisos, %v; quer 0", n, err)
	}
	relogio.Avancar(24 * time.Hour) // exatamente 30 dias antes
	if n, err := svc.AvisarVencimentos(ctx); err != nil || n != 1 {
		t.Fatalf("a 30 dias: %d avisos, %v; quer 1", n, err)
	}
	relogio.Avancar(time.Hour)
	if n, _ := svc.AvisarVencimentos(ctx); n != 0 {
		t.Fatalf("rodar de novo avisou %d vezes; quer 0", n)
	}

	evs := eventosDeCertificado(t, s)
	if len(evs) != 1 {
		t.Fatalf("%d eventos no outbox; quer 1", len(evs))
	}
	quer := app.DadosCertificado{ContaID: "loja", Sujeito: "CN=loja", ImpressaoDigital: "fp-loja", Validade: vence, DiasRestantes: 30, Vencido: false}
	if evs[0] != quer {
		t.Fatalf("evento = %+v\nquer     %+v", evs[0], quer)
	}
}

func TestAvisaCertificadoJaVencidoQueNuncaFoiAvisado(t *testing.T) {
	s := fake.NewStore()
	svc := app.Certificados{Store: s, Clock: fake.NewClock(vence.Add(25 * time.Hour))} // o Lastro ficou desligado na janela
	comCertificado(t, s, "loja", vence, true)
	if n, err := svc.AvisarVencimentos(context.Background()); err != nil || n != 1 {
		t.Fatalf("%d avisos, %v; quer 1", n, err)
	}
	evs := eventosDeCertificado(t, s)
	if len(evs) != 1 || !evs[0].Vencido || evs[0].DiasRestantes != -1 {
		t.Fatalf("evento = %+v; quer vencido com -1 dia", evs)
	}
}

func TestNaoAvisaContaDesativadaNemSemCertificado(t *testing.T) {
	s := fake.NewStore()
	svc := app.Certificados{Store: s, Clock: fake.NewClock(vence.Add(-time.Hour))}
	comCertificado(t, s, "desativada", vence, false)
	if err := s.Contas().Criar(context.Background(), domain.Conta{ID: "sandbox", Nome: "S", PSP: "sandbox", ChavePix: "s@example.com", Ambiente: domain.Homologacao, Ativa: true}); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.AvisarVencimentos(context.Background()); err != nil || n != 0 {
		t.Fatalf("%d avisos, %v; quer 0", n, err)
	}
	if evs := eventosDeCertificado(t, s); len(evs) != 0 {
		t.Fatalf("eventos = %+v", evs)
	}
}

func TestAvisaCadaContaSeparada(t *testing.T) {
	s := fake.NewStore()
	relogio := fake.NewClock(vence.Add(-10 * 24 * time.Hour))
	svc := app.Certificados{Store: s, Clock: relogio}
	comCertificado(t, s, "a", vence, true)
	comCertificado(t, s, "b", vence.Add(60*24*time.Hour), true) // ainda longe
	if n, _ := svc.AvisarVencimentos(context.Background()); n != 1 {
		t.Fatalf("%d avisos; quer 1 (só a conta a)", n)
	}
	relogio.Avancar(40 * 24 * time.Hour) // agora b entra na janela
	if n, _ := svc.AvisarVencimentos(context.Background()); n != 1 {
		t.Fatalf("%d avisos; quer 1 (só a conta b)", n)
	}
	if evs := eventosDeCertificado(t, s); len(evs) != 2 || evs[0].ContaID == evs[1].ContaID {
		t.Fatalf("eventos = %+v", evs)
	}
}
