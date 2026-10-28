package brcode

import "fmt"

// crc16 calcula o CRC16-CCITT (polinômio 0x1021, valor inicial 0xFFFF) exigido
// pelo Manual de BR Code do BCB.
func crc16(data string) uint16 {
	crc := uint16(0xFFFF)
	for i := 0; i < len(data); i++ {
		crc ^= uint16(data[i]) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// comCRC acrescenta o campo 63 ao payload: "6304" + CRC em 4 dígitos hexadecimais maiúsculos.
func comCRC(payload string) string {
	payload += "6304"
	return payload + fmt.Sprintf("%04X", crc16(payload))
}
