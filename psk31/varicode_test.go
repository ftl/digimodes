package psk31

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVaricodeUniqueness(t *testing.T) {
	codes := make(map[Symbol]bool)
	for i, c := range Varicode {
		exists := codes[c]
		assert.False(t, exists, "code already exists %4x %d", c, i)
		codes[c] = true
	}
}
