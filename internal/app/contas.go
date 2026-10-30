// Package app contém os casos de uso que orquestram domínio e ports.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/vintemhq/vintem/internal/domain"
	"github.com/vintemhq/vintem/internal/ports"
)

// Contas gerencia as contas recebedoras e as credenciais delas (RN-03, RN-61, RN-62).
type Contas struct {
	Store        ports.Store
	Vault        ports.Vault
	PSPs         ports.Registry
	Certificados ports.Certificados
	Clock        ports.Clock
}

// Criar valida a conta, confere se o PSP existe e se o certificado é legível e
// está no prazo, e grava a conta com as credenciais cifradas, numa transação só.
// Os dados públicos do certificado ficam na conta. Conta nasce ativa.
func (s Contas) Criar(ctx context.Context, c domain.Conta, cred domain.Credenciais) error {
	c.Ativa = true
	c.Certificado = nil
	if err := c.Validar(); err != nil {
		return err
	}
	if _, err := s.PSPs.PSP(c.PSP); err != nil {
		return err
	}
	if len(cred.Certificado) > 0 {
		cert, err := s.lerCertificado(cred)
		if err != nil {
			return err
		}
		c.Certificado = &cert
	}
	plano, err := json.Marshal(cred) //nolint:gosec // G117: serializado só para ser cifrado na linha seguinte
	if err != nil {
		return fmt.Errorf("serializando credenciais: %w", err)
	}
	cifrado, err := s.Vault.Cifrar(ctx, plano)
	if err != nil {
		return fmt.Errorf("cifrando credenciais: %w", err)
	}
	return s.Store.WithTx(ctx, func(tx ports.Store) error {
		if err := certificadoServeNoAmbiente(ctx, tx, c); err != nil {
			return err
		}
		if err := tx.Contas().Criar(ctx, c); err != nil {
			return err
		}
		return tx.Contas().SalvarCredenciais(ctx, c.ID, cifrado)
	})
}

// certificadoServeNoAmbiente recusa, em conta de produção, um certificado que já
// está numa conta de homologação (RN-62). Nada no X.509 diz o ambiente; o que
// vale é onde o certificado já foi usado, pela impressão digital.
func certificadoServeNoAmbiente(ctx context.Context, tx ports.Store, c domain.Conta) error {
	if c.Ambiente != domain.Producao || c.Certificado == nil {
		return nil
	}
	contas, err := tx.Contas().Listar(ctx)
	if err != nil {
		return err
	}
	for _, outra := range contas {
		if outra.Ambiente == domain.Homologacao && outra.Certificado != nil && outra.Certificado.ImpressaoDigital == c.Certificado.ImpressaoDigital {
			return fmt.Errorf("%w: este certificado já está na conta de homologação %s; use o certificado que o banco emitiu para produção", domain.ErrCertificadoDeHomologacao, outra.ID)
		}
	}
	return nil
}

func (s Contas) lerCertificado(cred domain.Credenciais) (domain.Certificado, error) {
	if s.Certificados == nil || s.Clock == nil {
		return domain.Certificado{}, errors.New("contas: Certificados e Clock são obrigatórios para conferir o certificado")
	}
	cert, err := s.Certificados.Ler(cred.Certificado, cred.SenhaCertificado)
	if err != nil {
		return domain.Certificado{}, err
	}
	if cert.VencidoEm(s.Clock.Now()) {
		return domain.Certificado{}, fmt.Errorf("%w em %s (%s)", domain.ErrCertificadoVencido, cert.Validade.Format(time.DateOnly), cert.Sujeito)
	}
	return cert, nil
}

var _ ports.Credenciais = Contas{}

// PreencherCertificados grava os dados públicos do certificado das contas que
// têm certificado nas credenciais mas não têm os dados na conta: as cadastradas
// antes do v0.3.0, quando o certificado não era lido no cadastro (#79). Sem os
// dados, a conta não recebe o aviso de vencimento nem entra na regra de ambiente.
//
// Certificado vencido é gravado mesmo assim, para o aviso sair. Certificado
// ilegível gera um aviso no log com o id e o motivo, sem nada do conteúdo, e não
// impede as outras contas. Conta já preenchida não é tocada: rodar de novo não
// refaz nada. Devolve quantas contas foram preenchidas.
func (s Contas) PreencherCertificados(ctx context.Context) (int, error) {
	if s.Certificados == nil {
		return 0, errors.New("contas: Certificados é obrigatório para preencher os certificados")
	}
	lista, err := s.Store.Contas().Listar(ctx)
	if err != nil {
		return 0, err
	}
	preenchidas := 0
	for _, c := range lista {
		if c.Certificado != nil {
			continue
		}
		cred, err := s.Credenciais(ctx, c.ID)
		if errors.Is(err, ports.ErrNaoEncontrado) {
			continue // conta sem credenciais, como a do sandbox
		}
		if err != nil {
			slog.WarnContext(ctx, "certificado da conta não pôde ser lido: credenciais ilegíveis", "conta", c.ID, "erro", err)
			continue
		}
		if len(cred.Certificado) == 0 {
			continue
		}
		cert, err := s.Certificados.Ler(cred.Certificado, cred.SenhaCertificado)
		if err != nil {
			slog.WarnContext(ctx, "certificado da conta não pôde ser lido; recadastre a conta com um certificado válido", "conta", c.ID, "erro", err)
			continue
		}
		if err := s.Store.Contas().AtualizarCertificado(ctx, c.ID, cert); err != nil {
			return preenchidas, fmt.Errorf("gravando o certificado da conta %s: %w", c.ID, err)
		}
		preenchidas++
	}
	return preenchidas, nil
}

// Desativar impede novas cobranças na conta; as existentes continuam valendo.
func (s Contas) Desativar(ctx context.Context, id string) error {
	return s.Store.WithTx(ctx, func(tx ports.Store) error {
		c, err := tx.Contas().Buscar(ctx, id)
		if err != nil {
			return err
		}
		c.Ativa = false
		return tx.Contas().Atualizar(ctx, c)
	})
}

// Buscar devolve uma conta, sem credenciais.
func (s Contas) Buscar(ctx context.Context, id string) (domain.Conta, error) {
	return s.Store.Contas().Buscar(ctx, id)
}

// Listar devolve as contas, sem credenciais.
func (s Contas) Listar(ctx context.Context) ([]domain.Conta, error) {
	return s.Store.Contas().Listar(ctx)
}

// ConferirChaveMestra decifra as credenciais das contas e os segredos dos
// destinos. Uma chave válida, mas diferente, passaria despercebida até a primeira
// chamada ao banco; assim, o erro aparece ao abrir o banco.
func (s Contas) ConferirChaveMestra(ctx context.Context) error {
	return s.percorrerSegredos(ctx, s.Store, func(string, []byte) error { return nil }, func(string, []byte) error { return nil })
}

// TrocarChaveMestra recifra as credenciais e os segredos com a chave nova, numa
// transação só: ou tudo passa para a nova, ou nada muda. Devolve quantas contas e
// quantos destinos foram recifrados.
func (s Contas) TrocarChaveMestra(ctx context.Context, nova ports.Vault) (contas, destinos int, err error) {
	err = s.Store.WithTx(ctx, func(tx ports.Store) error {
		contas, destinos = 0, 0
		return s.percorrerSegredos(ctx, tx, func(id string, plano []byte) error {
			cifrado, err := nova.Cifrar(ctx, plano)
			if err != nil {
				return fmt.Errorf("cifrando as credenciais da conta %s: %w", id, err)
			}
			contas++
			return tx.Contas().SalvarCredenciais(ctx, id, cifrado)
		}, func(id string, plano []byte) error {
			cifrado, err := nova.Cifrar(ctx, plano)
			if err != nil {
				return fmt.Errorf("cifrando o segredo do destino %s: %w", id, err)
			}
			destinos++
			return tx.Destinos().AtualizarSegredo(ctx, id, cifrado)
		})
	})
	return contas, destinos, err
}

// percorrerSegredos decifra cada segredo guardado e o entrega à função da conta
// ou do destino. Conta sem credenciais fica de fora.
func (s Contas) percorrerSegredos(ctx context.Context, store ports.Store, daConta, doDestino func(id string, plano []byte) error) error {
	lista, err := store.Contas().Listar(ctx)
	if err != nil {
		return err
	}
	for _, c := range lista {
		cifrado, err := store.Contas().BuscarCredenciais(ctx, c.ID)
		if errors.Is(err, ports.ErrNaoEncontrado) {
			continue
		}
		if err != nil {
			return err
		}
		plano, err := s.Vault.Decifrar(ctx, cifrado)
		if err != nil {
			return fmt.Errorf("credenciais da conta %s: %w", c.ID, err)
		}
		if err := daConta(c.ID, plano); err != nil {
			return err
		}
	}
	destinos, err := store.Destinos().Listar(ctx)
	if err != nil {
		return err
	}
	for _, d := range destinos {
		plano, err := s.Vault.Decifrar(ctx, d.SegredoCifrado)
		if err != nil {
			return fmt.Errorf("segredo do destino %s: %w", d.ID, err)
		}
		if err := doDestino(d.ID, plano); err != nil {
			return err
		}
	}
	return nil
}

// Credenciais decifra as credenciais da conta, para uso exclusivo do adapter de PSP.
func (s Contas) Credenciais(ctx context.Context, id string) (domain.Credenciais, error) {
	var cred domain.Credenciais
	cifrado, err := s.Store.Contas().BuscarCredenciais(ctx, id)
	if err != nil {
		return cred, err
	}
	plano, err := s.Vault.Decifrar(ctx, cifrado)
	if err != nil {
		return cred, fmt.Errorf("credenciais da conta %s: %w", id, err)
	}
	if err := json.Unmarshal(plano, &cred); err != nil {
		return cred, fmt.Errorf("credenciais da conta %s corrompidas: %w", id, err)
	}
	return cred, nil
}
