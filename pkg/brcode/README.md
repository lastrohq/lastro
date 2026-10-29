# brcode

Gera e lê BR Codes do Pix (o "Pix Copia e Cola" e o conteúdo do QR Code), no
padrão EMV MPM do [Manual de BR Code do BCB][manual], com CRC16-CCITT.

Só usa a biblioteca padrão e não depende do resto do Lastro:

```sh
go get github.com/lastrohq/lastro/pkg/brcode
```

## Gerar

```go
code, err := brcode.Encode(brcode.Pix{
	Chave:     "fulano@example.com",
	Nome:      "Padaria São João",
	Cidade:    "Lavras",
	Valor:     1590, // centavos: R$ 15,90
	Descricao: "Pedido 42",
	Txid:      "PEDIDO42",
})
// 00020126530014br.gov.bcb.pix0118fulano@example.com0209Pedido 42520400005303986540515.905802BR5916Padaria Sao Joao6006Lavras62120508PEDIDO426304BC9F
```

O `code` é o texto do Pix Copia e Cola. Para virar QR, passe-o para qualquer
gerador de QR Code. O pacote não desenha a imagem.

| Campo | Regra |
|---|---|
| `Chave` ou `URL` | Um dos dois, nunca ambos. `Chave` gera o estático; `URL` (location do PSP, sem `https://`) gera o dinâmico |
| `Valor` | Centavos em `int64`. Zero deixa o pagador digitar o valor |
| `Nome` | Até 25 caracteres. Acentos são removidos |
| `Cidade` | Até 15 caracteres. Acentos são removidos |
| `Txid` | Só no estático: até 25 alfanuméricos. Vazio vira `***`. No dinâmico o txid fica na location |
| `Descricao` | Só no estático. Chave e descrição somam no máximo 99 caracteres no campo 26 |
| `UnicoUso` | Marca o QR como de uso único (campo 01 = `12`) |

Dado fora das regras retorna erro que embrulha `brcode.ErrInvalido`.

**Por que tirar os acentos:** vários apps de banco recusam caracteres fora do
ASCII em nome e cidade. Além disso, o pagador não vê esses campos. O app mostra
o nome do titular cadastrado na chave (DICT), e nenhum app testado mostrou a
cidade.

## Ler

```go
p, err := brcode.Decode(code)
// p.Chave == "fulano@example.com", p.Valor == 1590, p.Txid == "PEDIDO42", p.Dinamico() == false
```

`Decode` confere o CRC e os campos obrigatórios antes de devolver qualquer coisa:

```go
_, err := brcode.Decode(codeComUmDigitoTrocado)
// br code inválido: CRC BC90 não confere (calculado BC9F)
```

## Compatibilidade com apps de banco

Teste manual em 29/10/2026 ([#9][issue]). Foram gerados BR Codes estáticos
com uma chave real e lidos em cada app até a tela de confirmação.

| App | Sem valor | Com valor | Descrição (02) | Txid (62.05) | Nome e cidade com acento na entrada |
|---|---|---|---|---|---|
| Nubank | ✅ | ✅ fixo | ✅ "Mensagem" | ✅ "Identificador" | ✅ |
| Santander | ✅ | ✅ fixo | ✅ "Informação adicional" | ✅ "Identificação do pagamento" | ✅ |
| Mercado Pago | ✅ | ✅ fixo | ✅ "Informação adicional" | não exibido | ✅ |
| Wise | ✅ | ✅ fixo¹ | não exibida | não exibido | ✅ |

¹ O Wise recusa Pix abaixo de R$ 0,10. Essa é uma regra do app: o valor foi
lido corretamente.

O BR Code **dinâmico** ainda não tem teste de ponta a ponta. Os quatro apps
reconheceram o código como Pix e tentaram buscar a location. Como a URL era
fictícia, todos falharam ao buscá-la, que é o resultado esperado. O teste
completo depende de uma location de PSP real (F5).

[manual]: https://www.bcb.gov.br/content/estabilidadefinanceira/spb_docs/ManualBRCode.pdf
[issue]: https://github.com/lastrohq/lastro/issues/9
