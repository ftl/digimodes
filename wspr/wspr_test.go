package wspr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	packedDB0ABC    = uint32(94281599)
	packedJN59Pwr12 = uint32(1953228)
)

func TestAlignCallsign(t *testing.T) {
	testCases := []struct {
		desc     string
		value    string
		valid    bool
		expected string
	}{
		{"too long", "dl1abcd", false, ""},
		{"too long after padding", "g9abcd", false, ""},
		{"number at wrong place", "9ab", false, ""},
		{"number in the suffix", "dl9000", false, ""},
		{"valid, 2 prefix, 3 suffix", "dl1abc", true, "DL1ABC"},
		{"valid, 2 prefix, 2 suffix", "dl1ab", true, "DL1AB "},
		{"valid, 1 prefix, 2 suffix", "g1ab", true, " G1AB "},
		{"valid, 2 prefix, 2 suffix", "9a1ab", true, "9A1AB "},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			actual, err := alignCallsign(tC.value)
			if tC.valid {
				assert.NoError(t, err)
				assert.Equal(t, tC.expected, string(actual))
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPackCallsign(t *testing.T) {
	packed, err := packCallsign("DB0ABC")
	assert.NoError(t, err)
	assert.Equal(t, packedDB0ABC, packed)
}

func TestPackLocatorAndPower(t *testing.T) {
	packedLocator, err := packLocator("JN59")
	require.NoError(t, err)

	packed := packPower(packedLocator, 12)
	assert.Equal(t, packedJN59Pwr12, packed)
}

func TestCompress(t *testing.T) {
	expected := [11]byte{0x59, 0xE9, 0xF7, 0xF7, 0x73, 0x73, 0x00, 0x00, 0x00, 0x00, 0x00}
	compressed := compress(packedDB0ABC, packedJN59Pwr12)
	assert.Equal(t, expected, compressed)
}

func TestCalcParity(t *testing.T) {
	expected := [162]byte{
		0, 0, 1, 1, 0, 1, 1, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 0, 1, 0, 1, 0, 0, 1, 0, 1, 0, 0, 0, 1, 1, 0,
		0, 1, 0, 0, 1, 0, 0, 1, 1, 1, 1, 0, 1, 1, 1, 0, 0, 1, 0, 1, 0, 1, 0, 0, 0, 1, 0, 0, 1, 0, 1, 1,
		1, 0, 1, 1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0, 1, 0, 0, 1, 0,
		1, 1, 0, 1, 0, 1, 0, 1, 0, 0, 0, 1, 1, 0, 1, 1, 0, 0, 0, 1, 1, 1, 0, 1, 0, 1, 1, 0, 1, 0, 0, 1,
		0, 0, 0, 0, 1, 1, 0, 1, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 0,
		0, 0,
	}
	parity := calcParity(compress(packedDB0ABC, packedJN59Pwr12))
	assert.Equal(t, expected, parity)
}

func TestInterleave(t *testing.T) {
	expected := [162]byte{
		0, 1, 1, 1, 0, 0, 0, 0, 0, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1,
		1, 1, 1, 0, 0, 0, 1, 0, 1, 0, 0, 1, 0, 1, 1, 0, 0, 0, 1, 0, 1, 1, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0,
		1, 1, 0, 1, 1, 0, 1, 0, 1, 1, 1, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0, 1, 1, 0,
		1, 1, 1, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 1, 1, 1, 1, 1, 0, 0, 1, 1, 0, 1, 1, 1, 1, 0, 1, 1, 1, 0,
		0, 1, 1, 0, 0, 1, 1, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 0, 1, 1, 0, 0, 0, 1, 1, 0, 0, 1, 0,
		0, 1,
	}
	interleaved := interleave(calcParity(compress(packedDB0ABC, packedJN59Pwr12)))
	assert.Equal(t, expected, interleaved)
}

func TestSynchronize(t *testing.T) {
	expected := Transmission{
		Sym1, Sym3, Sym2, Sym2, Sym0, Sym0, Sym0, Sym0, Sym1, Sym0, Sym2, Sym2, Sym1, Sym1, Sym3, Sym0, Sym2, Sym2, Sym3, Sym2, Sym0, Sym3, Sym2, Sym3, Sym1, Sym3, Sym1, Sym2, Sym2, Sym2, Sym2, Sym2,
		Sym2, Sym2, Sym3, Sym0, Sym0, Sym1, Sym2, Sym1, Sym2, Sym0, Sym0, Sym2, Sym0, Sym2, Sym3, Sym0, Sym1, Sym1, Sym2, Sym0, Sym3, Sym3, Sym2, Sym1, Sym0, Sym0, Sym0, Sym1, Sym3, Sym0, Sym1, Sym0,
		Sym2, Sym2, Sym0, Sym3, Sym3, Sym0, Sym3, Sym0, Sym3, Sym2, Sym3, Sym0, Sym3, Sym0, Sym0, Sym1, Sym2, Sym0, Sym1, Sym0, Sym1, Sym1, Sym2, Sym2, Sym2, Sym3, Sym1, Sym0, Sym1, Sym2, Sym3, Sym0,
		Sym2, Sym2, Sym3, Sym0, Sym2, Sym0, Sym0, Sym0, Sym1, Sym2, Sym0, Sym1, Sym0, Sym2, Sym3, Sym3, Sym3, Sym2, Sym1, Sym1, Sym2, Sym2, Sym1, Sym3, Sym2, Sym3, Sym2, Sym0, Sym2, Sym3, Sym3, Sym1,
		Sym0, Sym2, Sym2, Sym0, Sym0, Sym3, Sym2, Sym1, Sym0, Sym0, Sym1, Sym3, Sym0, Sym0, Sym2, Sym0, Sym0, Sym2, Sym0, Sym1, Sym1, Sym2, Sym3, Sym0, Sym1, Sym1, Sym2, Sym2, Sym0, Sym1, Sym3, Sym0,
		Sym0, Sym2,
	}
	transmission := synchronize(interleave(calcParity(compress(packedDB0ABC, packedJN59Pwr12))))
	assert.Equal(t, expected, transmission)
}

func TestToTransmission(t *testing.T) {
	expected := Transmission{
		Sym1, Sym3, Sym2, Sym2, Sym0, Sym0, Sym0, Sym0, Sym1, Sym0, Sym2, Sym2, Sym1, Sym1, Sym3, Sym0, Sym2, Sym2, Sym3, Sym2, Sym0, Sym3, Sym2, Sym3, Sym1, Sym3, Sym1, Sym2, Sym2, Sym2, Sym2, Sym2,
		Sym2, Sym2, Sym3, Sym0, Sym0, Sym1, Sym2, Sym1, Sym2, Sym0, Sym0, Sym2, Sym0, Sym2, Sym3, Sym0, Sym1, Sym1, Sym2, Sym0, Sym3, Sym3, Sym2, Sym1, Sym0, Sym0, Sym0, Sym1, Sym3, Sym0, Sym1, Sym0,
		Sym2, Sym2, Sym0, Sym3, Sym3, Sym0, Sym3, Sym0, Sym3, Sym2, Sym3, Sym0, Sym3, Sym0, Sym0, Sym1, Sym2, Sym0, Sym1, Sym0, Sym1, Sym1, Sym2, Sym2, Sym2, Sym3, Sym1, Sym0, Sym1, Sym2, Sym3, Sym0,
		Sym2, Sym2, Sym3, Sym0, Sym2, Sym0, Sym0, Sym0, Sym1, Sym2, Sym0, Sym1, Sym0, Sym2, Sym3, Sym3, Sym3, Sym2, Sym1, Sym1, Sym2, Sym2, Sym1, Sym3, Sym2, Sym3, Sym2, Sym0, Sym2, Sym3, Sym3, Sym1,
		Sym0, Sym2, Sym2, Sym0, Sym0, Sym3, Sym2, Sym1, Sym0, Sym0, Sym1, Sym3, Sym0, Sym0, Sym2, Sym0, Sym0, Sym2, Sym0, Sym1, Sym1, Sym2, Sym3, Sym0, Sym1, Sym1, Sym2, Sym2, Sym0, Sym1, Sym3, Sym0,
		Sym0, Sym2,
	}
	transmission, err := ToTransmission("DB0ABC", "JN59", 12)
	assert.NoError(t, err)
	assert.Equal(t, expected, transmission)
}
