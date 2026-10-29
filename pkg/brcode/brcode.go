package brcode

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// gui é o identificador do arranjo Pix no campo 26 (Merchant Account Information).
const gui = "br.gov.bcb.pix"

// txidLivre é o valor do campo 62.05 quando a cobrança não tem txid.
const txidLivre = "***"

// ErrInvalido indica um BR Code (ou os dados para gerá-lo) fora do Manual de BR Code.
var ErrInvalido = errors.New("br code inválido")

// Pix descreve um BR Code. Estático tem Chave; dinâmico tem URL. Nunca os dois.
type Pix struct {
	// Chave Pix do recebedor (BR Code estático).
	Chave string
	// Descricao é a informação adicional mostrada ao pagador (só estático).
	Descricao string
	// URL é a location do PSP, sem "https://" (BR Code dinâmico).
	URL string
	// Valor em centavos; zero deixa o valor para o pagador digitar.
	Valor int64
	// Nome do recebedor, até 25 caracteres.
	Nome string
	// Cidade do recebedor, até 15 caracteres.
	Cidade string
	// Txid do campo 62.05: até 25 alfanuméricos no estático; vazio vira "***".
	Txid string
	// UnicoUso marca o QR como de uso único (campo 01 = "12").
	UnicoUso bool
}

// Dinamico informa se o BR Code aponta para uma location do PSP.
func (p Pix) Dinamico() bool { return p.URL != "" }

// Encode gera o "Pix Copia e Cola". Nome e cidade têm acentos removidos, porque
// vários apps de banco não leem caracteres fora do ASCII.
func Encode(p Pix) (string, error) {
	p.Nome = semAcento(strings.TrimSpace(p.Nome))
	p.Cidade = semAcento(strings.TrimSpace(p.Cidade))
	if err := validar(p); err != nil {
		return "", err
	}

	conta := tlv("00", gui)
	if p.Dinamico() {
		conta += tlv("25", p.URL)
	} else {
		conta += tlv("01", p.Chave)
		if p.Descricao != "" {
			conta += tlv("02", p.Descricao)
		}
	}
	if len(conta) > 99 {
		return "", fmt.Errorf("%w: chave/URL e descrição somam %d caracteres no campo 26 (máximo 99)", ErrInvalido, len(conta))
	}

	var b strings.Builder
	b.WriteString(tlv("00", "01"))
	if p.UnicoUso {
		b.WriteString(tlv("01", "12"))
	}
	b.WriteString(tlv("26", conta))
	b.WriteString(tlv("52", "0000"))
	b.WriteString(tlv("53", "986"))
	if p.Valor > 0 {
		b.WriteString(tlv("54", fmt.Sprintf("%d.%02d", p.Valor/100, p.Valor%100)))
	}
	b.WriteString(tlv("58", "BR"))
	b.WriteString(tlv("59", p.Nome))
	b.WriteString(tlv("60", p.Cidade))
	txid := p.Txid
	if txid == "" {
		txid = txidLivre
	}
	b.WriteString(tlv("62", tlv("05", txid)))
	return comCRC(b.String()), nil
}

func validar(p Pix) error {
	switch {
	case p.Chave == "" && p.URL == "":
		return fmt.Errorf("%w: informe a chave (estático) ou a URL (dinâmico)", ErrInvalido)
	case p.Chave != "" && p.URL != "":
		return fmt.Errorf("%w: chave e URL são excludentes", ErrInvalido)
	case p.Dinamico() && p.Descricao != "":
		return fmt.Errorf("%w: descrição só existe no BR Code estático", ErrInvalido)
	case strings.Contains(p.URL, "://"):
		return fmt.Errorf("%w: a URL vai sem o esquema (sem https://)", ErrInvalido)
	case len(p.Chave) > 77:
		return fmt.Errorf("%w: chave com mais de 77 caracteres", ErrInvalido)
	case p.Valor < 0 || p.Valor > 999_999_999_999:
		return fmt.Errorf("%w: valor fora do intervalo", ErrInvalido)
	case p.Nome == "" || len(p.Nome) > 25 || !asciiImprimivel(p.Nome):
		return fmt.Errorf("%w: nome deve ter de 1 a 25 caracteres ASCII", ErrInvalido)
	case p.Cidade == "" || len(p.Cidade) > 15 || !asciiImprimivel(p.Cidade):
		return fmt.Errorf("%w: cidade deve ter de 1 a 15 caracteres ASCII", ErrInvalido)
	}
	if p.Txid != "" && p.Txid != txidLivre {
		if len(p.Txid) > 25 || !alfanumerico(p.Txid) {
			return fmt.Errorf("%w: txid do BR Code deve ter até 25 caracteres alfanuméricos", ErrInvalido)
		}
		if p.Dinamico() {
			return fmt.Errorf("%w: no BR Code dinâmico o txid fica na location, e o campo 62.05 é ***", ErrInvalido)
		}
	}
	return nil
}

// Decode interpreta um "Pix Copia e Cola", conferindo o CRC e os campos obrigatórios.
func Decode(s string) (Pix, error) {
	var p Pix
	if len(s) < 8 || s[len(s)-8:len(s)-4] != "6304" {
		return p, fmt.Errorf("%w: não termina com o campo 63 (CRC)", ErrInvalido)
	}
	crc, err := strconv.ParseUint(s[len(s)-4:], 16, 16)
	if err != nil {
		return p, fmt.Errorf("%w: CRC não é hexadecimal", ErrInvalido)
	}
	if got := crc16(s[:len(s)-4]); uint16(crc) != got {
		return p, fmt.Errorf("%w: CRC %s não confere (calculado %04X)", ErrInvalido, s[len(s)-4:], got)
	}

	campos, err := lerTLV(s[:len(s)-8])
	if err != nil {
		return p, err
	}
	if campos["00"] != "01" {
		return p, fmt.Errorf("%w: campo 00 deve ser 01", ErrInvalido)
	}
	p.UnicoUso = campos["01"] == "12"

	conta, err := lerTLV(campos["26"])
	if err != nil {
		return p, fmt.Errorf("campo 26: %w", err)
	}
	if !strings.EqualFold(conta["00"], gui) {
		return p, fmt.Errorf("%w: campo 26 não é do arranjo Pix", ErrInvalido)
	}
	p.Chave, p.Descricao, p.URL = conta["01"], conta["02"], conta["25"]
	if (p.Chave == "") == (p.URL == "") {
		return p, fmt.Errorf("%w: campo 26 deve ter chave ou URL, e só um deles", ErrInvalido)
	}

	if campos["53"] != "986" || campos["58"] != "BR" {
		return p, fmt.Errorf("%w: moeda deve ser 986 e país BR", ErrInvalido)
	}
	if v, ok := campos["54"]; ok {
		if p.Valor, err = lerValor(v); err != nil {
			return p, err
		}
	}
	p.Nome, p.Cidade = campos["59"], campos["60"]
	if p.Nome == "" || p.Cidade == "" {
		return p, fmt.Errorf("%w: nome e cidade são obrigatórios", ErrInvalido)
	}
	if adicional, ok := campos["62"]; ok {
		extra, err := lerTLV(adicional)
		if err != nil {
			return p, fmt.Errorf("campo 62: %w", err)
		}
		if extra["05"] != txidLivre {
			p.Txid = extra["05"]
		}
	}
	return p, nil
}

func tlv(id, valor string) string { return fmt.Sprintf("%s%02d%s", id, len(valor), valor) }

func lerTLV(s string) (map[string]string, error) {
	campos := make(map[string]string)
	for i := 0; i < len(s); {
		if i+4 > len(s) {
			return nil, fmt.Errorf("%w: campo truncado na posição %d", ErrInvalido, i)
		}
		id := s[i : i+2]
		n, err := strconv.Atoi(s[i+2 : i+4])
		if err != nil || n < 0 || !digitos(s[i+2:i+4]) {
			return nil, fmt.Errorf("%w: tamanho inválido no campo %s", ErrInvalido, id)
		}
		if i+4+n > len(s) {
			return nil, fmt.Errorf("%w: campo %s maior que o restante do código", ErrInvalido, id)
		}
		if _, repetido := campos[id]; repetido {
			return nil, fmt.Errorf("%w: campo %s repetido", ErrInvalido, id)
		}
		campos[id] = s[i+4 : i+4+n]
		i += 4 + n
	}
	return campos, nil
}

// lerValor aceita o formato do campo 54 ("10", "10.5", "10.50") e devolve centavos.
func lerValor(v string) (int64, error) {
	inteiro, frac, ponto := strings.Cut(v, ".")
	if !digitos(inteiro) || (ponto && (len(frac) == 0 || len(frac) > 2 || !digitos(frac))) || len(v) > 13 {
		return 0, fmt.Errorf("%w: valor %q", ErrInvalido, v)
	}
	reais, _ := strconv.ParseInt(inteiro, 10, 64)
	frac = (frac + "00")[:2]
	centavos, _ := strconv.ParseInt(frac, 10, 64)
	return reais*100 + centavos, nil
}

func digitos(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func alfanumerico(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func asciiImprimivel(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

var acentos = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "Á", "A", "À", "A", "Â", "A", "Ã", "A", "Ä", "A",
	"é", "e", "è", "e", "ê", "e", "ë", "e", "É", "E", "È", "E", "Ê", "E", "Ë", "E",
	"í", "i", "ì", "i", "î", "i", "ï", "i", "Í", "I", "Ì", "I", "Î", "I", "Ï", "I",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o", "Ó", "O", "Ò", "O", "Ô", "O", "Õ", "O", "Ö", "O",
	"ú", "u", "ù", "u", "û", "u", "ü", "u", "Ú", "U", "Ù", "U", "Û", "U", "Ü", "U",
	"ç", "c", "Ç", "C", "ñ", "n", "Ñ", "N",
)

func semAcento(s string) string { return acentos.Replace(s) }
