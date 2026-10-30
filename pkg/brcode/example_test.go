package brcode_test

import (
	"fmt"

	"github.com/vintemhq/vintem/pkg/brcode"
)

func ExampleEncode() {
	code, err := brcode.Encode(brcode.Pix{
		Chave:  "123e4567-e12b-12d1-a456-426655440000",
		Nome:   "Fulano de Tal",
		Cidade: "BRASILIA",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(code)
	// Output: 00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D
}
