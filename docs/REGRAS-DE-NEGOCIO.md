# 📜 Lastro — Regras de negócio

O que o Lastro **garante**, o que ele **recusa** e por quê. Toda regra tem um
número estável (`RN-xx`) para ser citada em issue, teste e commit. Regra que
muda ganha nota com data; regra que cai fica riscada, não some.

> Os nomes de estado e campo seguem a [spec da API Pix do BCB](https://github.com/bacen/pix-api)
> (v2.10.0). Onde o Lastro estende a spec, isso está dito explicitamente.

---

## Princípios

### RN-01 · Não custodial

O Lastro nunca guarda, recebe nem movimenta dinheiro. Ele roda na
infraestrutura do dono da conta, com as credenciais do dono da conta, contra a
API Pix do banco do dono da conta. Nenhuma funcionalidade pode exigir que o
dinheiro passe por uma conta que não seja a do recebedor.

**Por quê:** é o que mantém o projeto fora do papel de instituição de pagamento.
A tese de que software não custodial dispensa autorização do BCB é uma
**inferência**, não norma encontrada — por isso o README traz o aviso de que
nada aqui é aconselhamento regulatório, e uma eventual versão hospedada exige
análise jurídica própria antes de existir.

### RN-02 · Dinheiro é inteiro, em centavos

Valores trafegam e são armazenados como `int64` em centavos. `float` não
aparece em nenhuma camada. A spec do BCB usa string decimal (`"10.50"`); a
conversão acontece só na borda (adapter e API) e recusa mais de duas casas,
sinal negativo e separador de milhar.

### RN-03 · Uma instância, N contas

Uma instância do Lastro atende várias **contas recebedoras**. Conta = PSP +
chave Pix + credenciais (client id/secret e certificado) + ambiente
(homologação ou produção). Toda cobrança pertence a exatamente uma conta.

---

## Cobranças

### RN-10 · txid

- Em `cob` e `cobv`: de **26 a 35** caracteres, `[a-zA-Z0-9]`.
- Se o cliente não informar, o Lastro gera (aleatório criptográfico, 32 caracteres).
- Se informar, o Lastro valida antes de chamar o banco — erro de formato nunca
  vira chamada de rede.
- txid é único **por conta**.

### RN-11 · Idempotência

Toda criação (`PUT`/`POST` que cria recurso) aceita o header `Idempotency-Key`.

- Mesma chave + mesmo payload em até **24h** → devolve a resposta original, sem
  chamar o banco de novo.
- Mesma chave + payload diferente → **409**.
- Criação sem a chave é aceita no `PUT /cob/{txid}` (o txid já é idempotente) e
  recusada no `POST /cob`.

### RN-12 · Estados da cobrança (`cob` e `cobv`)

```
ATIVA ──▶ CONCLUIDA
  │
  ├──▶ REMOVIDA_PELO_USUARIO_RECEBEDOR
  └──▶ REMOVIDA_PELO_PSP
```

Os três finais são **terminais**. Transição fora deste desenho é recusada no
domínio, antes de qualquer persistência. `CONCLUIDA` só é alcançada por
evidência de pagamento: webhook do banco ou conciliação (RN-31) — nunca por
chamada da API do cliente.

### RN-13 · Cobrança imediata (`cob`)

- Expiração padrão: **3600 s**, configurável por cobrança.
- Valor pode ser fixo ou permitir alteração pelo pagador (`modalidadeAlteracao`).
- Cobrança expirada continua `ATIVA` na spec do BCB; o Lastro expõe o campo
  derivado `expirada: true` (extensão) para não obrigar o cliente a calcular.

### RN-14 · Cobrança com vencimento (`cobv`)

- Exige devedor identificado (CPF ou CNPJ válido + nome).
- `dataDeVencimento` + `validadeAposVencimento` (dias corridos) definem até
  quando pode ser paga.
- Multa, juros, abatimento e desconto seguem as modalidades da spec; o Lastro
  **valida** as combinações, mas o cálculo final é do banco. O valor que vale é
  o que o banco liquidou.

### RN-15 · BR Code

- Gerado em conformidade com o padrão EMV MPM do Manual de BR Code do BCB, com
  CRC16-CCITT (`0xFFFF`, polinômio `0x1021`).
- Dinâmico (URL de location do banco) para `cob`/`cobv`; estático para uso
  avulso via `pkg/brcode`.
- O Lastro **nunca** gera BR Code dinâmico com location própria: a location é
  sempre a do PSP da conta.

---

## Devoluções

### RN-20 · Estados da devolução

```
EM_PROCESSAMENTO ──▶ DEVOLVIDO
                 └─▶ NAO_REALIZADO
```

### RN-21 · Limites

- A soma das devoluções de um Pix (`endToEndId`) não ultrapassa o valor recebido.
- Devolução identificada pelo par `e2eid` + `id` (id definido pelo cliente,
  idempotente).
- Prazo máximo de **90 dias** após o pagamento — validado antes de chamar o banco.

---

## Pix Automático

> ⚠️ **Pendente do estudo #1**: quais bancos expõem `/rec` e `/cobr` em
> produção para PJ. As regras abaixo descrevem o contrato da spec; a matriz de
> capacidades (RN-50) diz quem suporta.

### RN-40 · Recorrência (`/rec`)

```
CRIADA ──▶ APROVADA ──▶ CANCELADA
   │          └───────▶ EXPIRADA
   ├──▶ REJEITADA
   └──▶ EXPIRADA
```

- Uma recorrência tem periodicidade, valor (fixo ou máximo) e vigência.
- Só gera cobrança (`cobr`) quando `APROVADA`.

### RN-41 · Solicitação de confirmação (`/solicrec`)

É o pedido de autorização enviado ao pagador. Aprovação e rejeição chegam por
webhook de recorrência (`/webhookrec`) e movem a recorrência (RN-40).

### RN-42 · Cobrança recorrente (`/cobr`)

- Agendada com a antecedência mínima do normativo vigente.
- Tentativas e retentativas seguem o normativo vigente.

### RN-43 · Prazos versionados

Antecedência, janelas de retentativa e afins **não são constantes no código**.
Ficam numa tabela de parâmetros com **data do normativo** de origem, para que
cada mudança do BCB vire um commit pequeno, revisável e com fonte — e um
release com nota.

---

## Webhooks

### RN-30 · Entrada (banco → Lastro)

- Só é aceito webhook de origem configurada para a conta: mTLS (quando o banco
  exige, como o Efí) ou segredo na URL/header.
- O evento é **persistido antes** de responder 200. Se a persistência falhar, o
  Lastro responde 5xx e deixa o banco reenviar.
- Deduplicação por `endToEndId`: o mesmo Pix avisado duas vezes gera um evento só.

### RN-31 · Conciliação

Um job periódico consulta `GET /pix?inicio&fim` em cada conta (janela
deslizante, padrão a cada 5 min olhando as últimas 2 h) e trata como recebido
todo Pix que não chegou por webhook. **Nenhum Pix some** porque o banco falhou
em avisar.

### RN-32 · Saída (Lastro → aplicação do cliente)

- Payload **normalizado**: o mesmo formato, seja qual for o banco.
- Assinado conforme [Standard Webhooks](https://www.standardwebhooks.com/)
  (`webhook-id`, `webhook-timestamp`, `webhook-signature` HMAC-SHA256).
- Entrega *at-least-once*: o cliente deve deduplicar pelo `webhook-id`.
- Retry com backoff exponencial e jitter por até **24 h**; depois disso o evento
  vai para a **DLQ**, visível no painel e reenviável manualmente.
- Registro de cada tentativa (status, latência, corpo da resposta truncado).

---

## Segurança e acesso

### RN-60 · API keys

- Com escopo (`cob:read`, `cob:write`, `cobv:write`, `pix:read`, `devolucao:write`,
  `rec:write`, `webhook:admin`, `conta:admin`).
- Guardadas apenas como hash; o valor aparece **uma vez**, na criação.
- Podem ser restritas a uma conta.

### RN-61 · Segredos em repouso

Certificados, chaves privadas e client secrets são cifrados com AES-256-GCM
usando `LASTRO_MASTER_KEY`. Sem a master key, o Lastro não sobe. Nada disso é
escrito em log — nem mascarado parcialmente.

### RN-62 · Certificados

- Alerta (painel e webhook `certificado.expirando`) a **30 dias** do vencimento.
- Recusa usar em conta de produção um certificado marcado como de homologação
  (o Bradesco, por exemplo, não aceita o mesmo certificado nos dois).

---

## Adapters

### RN-50 · Matriz de capacidades

Cada adapter declara o que suporta: `cob`, `cobv`, `devolucao`, `webhook`,
`rec`, `cobr`, `lote`. Chamada a algo que o banco da conta não suporta responde
**501** com o nome da capacidade — nunca um erro genérico do banco.

### RN-51 · Conformidade

Adapter só entra no `develop` se passar na suíte de conformidade
(`test/conformance`) contra fixtures gravadas. Desvio de banco em relação à
spec é tratado **dentro** do adapter e documentado nele; o domínio nunca ganha
um `if banco == ...`.

### RN-52 · Sandbox

O adapter `sandbox` implementa todas as capacidades com estado em memória ou
SQLite: cobrança criada é paga após N segundos (configurável), pode simular
falha, expiração e webhook duplicado. É o adapter de referência da suíte.
