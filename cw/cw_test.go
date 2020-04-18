package cw

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteToSymbolStream(t *testing.T) {
	buf := make(chan Symbol, 1000)

	WriteToSymbolStream(context.Background(), buf, "Paris Paris")
	close(buf)

	symbols := make([]Symbol, 0, 56)
	weightSum := 0
	for s := range buf {
		symbols = append(symbols, s)
		weightSum += int(s.Weight)
	}

	assert.Equal(t, 56, len(symbols))
	assert.Equal(t, 100, weightSum)
}
