package psk31

import (
	"context"
	"time"
)

// WriteToEncodedStream writes the given text as varicode-encoded 16bit blocks to the given channel.
func WriteToEncodedStream(ctx context.Context, encoded chan<- uint16, text string) {
	for _, b := range text {
		select {
		case encoded <- Varicode[b&0x7F]:
		case <-ctx.Done():
			return
		}
	}
}

// Pack the incoming 16bit varicode blocks by trimming the trailing zeros.
func Pack(ctx context.Context, packed chan<- uint8, encoded <-chan uint16) {
	var out uint8
	lastWasZero := false
	outBit := 0
	for {
		select {
		case in := <-encoded:
			for i := 15; i >= 0; i-- {
				b := (in >> uint8(i)) & 0x0001
				out = (out << 1) | uint8(b)
				outBit = (outBit + 1) % 8

				if outBit == 0 {
					packed <- out
					out = 0
				}

				if lastWasZero && (b == 0) {
					break
				}
				lastWasZero = (b == 0)
			}
		case <-time.After(2 * time.Millisecond):
			if outBit != 0 {
				out = (out << uint8(8-outBit))
				packed <- out
				out = 0
				outBit = 0
			}
		case <-ctx.Done():
			return
		}
	}
}
