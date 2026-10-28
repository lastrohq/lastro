package brcode

import "testing"

// exemploManualBCB é o BR Code estático de exemplo do Manual de BR Code do BCB.
const exemploManualBCB = "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D"

func TestCRCDoExemploDoManual(t *testing.T) {
	semCRC := exemploManualBCB[:len(exemploManualBCB)-8]
	if got := comCRC(semCRC); got != exemploManualBCB {
		t.Fatalf("CRC = %s, quer %s", got[len(got)-4:], exemploManualBCB[len(exemploManualBCB)-4:])
	}
}

func TestCRC16CheckValue(t *testing.T) {
	// Valor de verificação padrão do CRC-16/CCITT-FALSE.
	if got := crc16("123456789"); got != 0x29B1 {
		t.Fatalf("crc16 = %04X, quer 29B1", got)
	}
}
