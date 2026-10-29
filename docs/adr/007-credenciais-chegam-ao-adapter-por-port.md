# ADR-007 · Credenciais chegam ao adapter de PSP por um port, montado depois do vault

**Status:** Aceita · **Data:** 29/10/2026

## Contexto

O adapter de um banco real precisa, a cada chamada, do client id/secret e do
certificado da conta, para o OAuth2 e o mTLS. As credenciais ficam cifradas
pelo Vault (RN-61) e só são decifradas por `app.Contas.Credenciais`. Só que o
adapter não pode importar `app` (regra de camadas), e os métodos do port `PSP`
recebem a `domain.Conta`, que de propósito não carrega segredo.

Para resolver, havia três caminhos:
- **Passar as credenciais em cada método do `PSP`:** obriga todo caso de uso a
  decifrar e carregar segredo, e espalha o segredo pelo núcleo.
- **Ligar o adapter às credenciais depois de construído:** deixa o adapter mutável
  e com uma janela em que fica sem credenciais.
- **Montar o registry depois do vault**, entregando um provedor de credenciais.

## Decisão

- Port novo `ports.Credenciais` (`Credenciais(ctx, contaID)`), que o `app.Contas`
  já implementa. O adapter recebe o provedor no construtor (`efi.New(credenciais, clock)`).
- `cli.Ambiente` ganha `MontarPSPs func(ports.Credenciais) ports.Registry`. A CLI
  monta o registry dentro de `abrir`, depois do store e do vault, e todos os
  casos de uso usam esse registry. Quando o campo é nil, como nos testes que só
  usam o sandbox, vale o `PSPs` fixo.
- O adapter decifra as credenciais a cada chamada (AES-GCM é barato) e guarda o
  cliente HTTP por conta, junto com um resumo das credenciais. Se as credenciais
  mudarem, o cliente é montado de novo e o token velho não é reusado.

## Consequências

- ➕ O segredo só existe em claro dentro do adapter, durante a chamada.
- ➕ O port `PSP` não mudou, e o sandbox e a suíte de conformidade continuam iguais.
- ➕ Trocar credenciais ou certificado vale na próxima chamada, sem reiniciar.
- ➖ O registry só existe com o store aberto, então quem usar o provedor depois de
  fechar recebe erro, e isso é o correto.
- ➖ Cada chamada ao banco decifra as credenciais. Se pesar, dá para guardar em
  cache por conta, junto com o cliente.
