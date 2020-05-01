package psk31

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSymbolPacker(t *testing.T) {
	testCases := []struct {
		desc     string
		input    []byte
		expected []uint8
	}{
		{"<empty>", []byte(""), []uint8{}},
		{"aaa", []byte("aaa"), []uint8{0b10110010, 0b11001011, 0}},
		{"aat", []byte("aat"), []uint8{0b10110010, 0b11001010, 0}},
		{"A", []byte("A"), []uint8{0b11111010, 0}},
		{"B", []byte("B"), []uint8{0b11101011, 0}},
		{"-", []byte("-"), []uint8{0b11010100}},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			packed := make(chan interface{}, len(tC.input)*2+2)
			packer := symbolPacker{}
			for _, s := range tC.input {
				packer.Pack(packed, Varicode[s])
			}
			packer.Pack(packed, make(endToken))
			close(packed)
			actual := make([]uint8, 0, len(tC.expected))
			for raw := range packed {
				s, ok := raw.(uint8)
				if ok {
					actual = append(actual, s)
				}
			}
			assert.Equal(t, tC.expected, actual)
		})
	}
}
