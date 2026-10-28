# ADR-004 · O Lastro é não custodial

**Status:** Aceita · **Data:** 28/10/2026

## Contexto

Intermediar recursos de terceiros exige autorização do BCB como instituição de
pagamento. O Lastro existe para que o recebedor fale direto com o próprio banco.

## Decisão

- O Lastro nunca recebe, guarda ou movimenta dinheiro; só chama a API do banco do próprio recebedor, com as credenciais dele.
- Nenhuma feature pode exigir conta intermediária (split, saldo, repasse ficam fora do escopo).
- O README e as docs trazem o aviso de que isso não é aconselhamento jurídico ou regulatório.
- Uma versão hospedada (SaaS) do Lastro **não** é coberta por esta ADR e exige análise jurídica antes de existir.

## Consequências

- ➕ Projeto fora do perímetro regulatório de IP, até onde a análise alcança.
- ➕ Narrativa simples: "seu dinheiro nunca passa por nós".
- ➖ A tese de dispensa de autorização é **inferência**, não norma encontrada — fica como risco aberto.
- ➖ Recusamos features que gateways oferecem (split, antecipação).
