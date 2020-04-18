/*
Package wspr implements the WSPR digital mode.

This implementation is based on G4JNT's description of the WSPR coding process: http://g4jnt.com/WSPR_Coding_Process.pdf.
Many thanks to Andy/G4JNT for working this out!
*/
package wspr

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// Send transmits the given transmission using the given functions to activate the transmitter and to transmit the symbol.
func Send(ctx context.Context, activateTransmitter func(bool), transmitSymbol func(Symbol), transmission Transmission) bool {
	defer activateTransmitter(false)
	if !waitForTransmitStart(ctx) {
		return false
	}

	log.Print("transmission start")

	for i, symbol := range transmission {
		fmt.Print(".")

		transmitSymbol(symbol)
		if i == 0 {
			activateTransmitter(true)
		}

		select {
		case <-time.After(SymbolDuration):
		case <-ctx.Done():
			return false
		}
	}

	fmt.Println()
	log.Print("transmission end")
	return true
}

func waitForTransmitStart(ctx context.Context) bool {
	for {
		log.Print("waiting for next transmission cycle")
		select {
		case <-ctx.Done():
			return false
		case now := <-time.After(1 * time.Second):
			if isTransmitStart(now) {
				return true
			}
		}
	}
}

func isTransmitStart(t time.Time) bool {
	return t.Minute()%2 == 0 && t.Second() == 0
}

// Symbol in WSPR. The value represents the delta to the base frequency.
type Symbol float64

const symbolDelta = float64(12000) / float64(8192)

// The four WSPR symbols.
const (
	Sym0 = Symbol(0.0 * symbolDelta)
	Sym1 = Symbol(1.0 * symbolDelta)
	Sym2 = Symbol(2.0 * symbolDelta)
	Sym3 = Symbol(3.0 * symbolDelta)
)

// Symbols contains all WSPR symbols.
var Symbols = []Symbol{Sym0, Sym1, Sym2, Sym3}

// SymbolDuration is the duration of one WSRP symbol.
var SymbolDuration = (8192 * 1000 / 12) * time.Microsecond

// Transmission of WSPR symbols.
type Transmission [162]Symbol

// ToTransmission converts the given data into a WSPR transmission.
func ToTransmission(callsign string, locator string, dBm int) (Transmission, error) {
	n, err := packCallsign(callsign)
	if err != nil {
		return Transmission{}, err
	}
	m, err := packLocator(locator)
	if err != nil {
		return Transmission{}, err
	}
	m = packPower(m, dBm)

	c := compress(n, m)
	parity := calcParity(c)
	interleaved := interleave(parity)
	transmission := synchronize(interleaved)

	return transmission, nil
}

func packCallsign(callsign string) (uint32, error) {
	if len(callsign) > 6 {
		return 0, errors.New("callsign too long (> 6)")
	}

	aligned, err := alignCallsign(callsign)
	if err != nil {
		return 0, err
	}

	packed := charValue(aligned[0])
	packed = packed*36 + charValue(aligned[1])
	packed = packed*10 + charValue(aligned[2])
	packed = packed*27 + (charValue(aligned[3]) - 10)
	packed = packed*27 + (charValue(aligned[4]) - 10)
	packed = packed*27 + (charValue(aligned[5]) - 10)
	packed = packed & 0x0FFFFFFF

	return packed, nil
}

func alignCallsign(callsign string) ([]byte, error) {
	aligned := callsign

	if isNumber(callsign[1]) {
		aligned = " " + aligned
	}
	if len(aligned) > 6 {
		return []byte{}, errors.New("callsign too long (> 6)")
	}
	for len(aligned) < 6 {
		aligned += " "
	}
	aligned = strings.ToUpper(aligned)

	if !(isNumber(aligned[0]) || isLetter(aligned[0]) || isSpace(aligned[0])) {
		return []byte{}, errors.New("wrong character at callsign start")
	}
	if !isLetter(aligned[1]) {
		return []byte{}, errors.New("callsign must have a letter in the prefix")
	}
	if !isNumber(aligned[2]) {
		return []byte{}, errors.New("callsign must have number at 2nd or 3rd place")
	}
	if !(isSuffix(aligned[3]) && isSuffix(aligned[4]) && isSuffix(aligned[5])) {
		return []byte{}, errors.New("callsign must only have letters in the suffix")
	}

	return []byte(aligned), nil
}

func packLocator(loc string) (uint32, error) {
	if len(loc) < 4 {
		return 0, errors.New("locator must have at least four characters")
	}

	normalized := strings.ToUpper(string(loc[0:4]))
	if !(isLocatorLetter(normalized[0]) && isLocatorLetter(normalized[1])) {
		return 0, errors.New("locator must have letters a the 1st and the 2nd position")
	}
	if !(isNumber(normalized[2]) && isNumber(normalized[3])) {
		return 0, errors.New("locator must have numbers at the 3rd and 4th position")
	}

	v := func(i int) uint32 {
		if i < 2 {
			return charValue(normalized[i]) - 10
		}
		return charValue(normalized[i])
	}

	packed := (179-10*v(0)-v(2))*180 + 10*v(1) + v(3)
	packed = packed & 0x00007FFF

	return packed, nil
}

func packPower(packedLocator uint32, dBm int) uint32 {
	return (packedLocator << 7) + uint32(dBm) + 64
}

func isNumber(b byte) bool {
	return b >= '0' && b <= '9'
}

func isLetter(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func isLocatorLetter(b byte) bool {
	return b >= 'A' && b <= 'R'
}

func isSpace(b byte) bool {
	return b == ' '
}

func isSuffix(b byte) bool {
	return isLetter(b) || isSpace(b)
}

func charValue(b byte) uint32 {
	switch {
	case isNumber(b):
		return uint32(b - '0')
	case isSpace(b):
		return 36
	default:
		return uint32(b-'A') + 10
	}
}

func compress(n, m uint32) (c [11]byte) {
	c[0] = byte((0x0FF00000 & n) >> 20)
	c[1] = byte((0x000FF000 & n) >> 12)
	c[2] = byte((0x00000FF0 & n) >> 4)
	c[3] = byte((0x0000000F&n)<<4) | byte((0x003C0000&m)>>18)
	c[4] = byte((0x0003FC00 & m) >> 10)
	c[5] = byte((0x000003FC & m) >> 2)
	c[6] = byte((0x00000003 & m) << 6)
	return
}

func calcParity(c [11]byte) (parity [162]byte) {
	const (
		polynom1 = uint32(0xf2d05351)
		polynom2 = uint32(0xe4613c47)
	)

	var (
		reg0, reg1 uint32
	)

	parityIndex := 0
	for i := 0; i < len(c); i++ {
		for j := 7; j >= 0; j-- {
			reg0 = (reg0 << 1) | uint32((c[i]>>uint8(j))&0x01)
			reg1 = reg0

			result0 := reg0 & polynom1
			result1 := reg1 & polynom2
			count0 := 0
			count1 := 0
			for k := 0; k < 32; k++ {
				if ((result0 >> uint8(k)) & 0x01) == 1 {
					count0++
				}
				if ((result1 >> uint8(k)) & 0x01) == 1 {
					count1++
				}
			}
			if count0%2 == 1 {
				parity[parityIndex] = 1
			}
			parityIndex++
			if count1%2 == 1 {
				parity[parityIndex] = 1
			}
			parityIndex++
		}
	}
	return
}

func interleave(parity [162]byte) (interleaved [162]byte) {
	p := 0
	for p < 162 {
		for k := 0; k <= 255; k++ {
			i := uint8(k)
			j := uint8(0)
			for l := 7; l >= 0; l-- {
				j |= (i & 0x01) << uint8(l)
				i = i >> 1
			}
			if j < 162 {
				interleaved[j] = parity[p]
				p++
			}
		}
	}
	return
}

func synchronize(interleaved [162]byte) (transmission Transmission) {
	syncWord := []byte{
		1, 1, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 1, 1, 0, 0, 0, 1, 0, 0, 1, 0, 1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 1, 0, 0,
		0, 0, 0, 0, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 0, 1, 1, 0, 1, 0, 0, 0, 0, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 0, 1, 0, 0, 1, 0,
		1, 1, 0, 0, 0, 1, 1, 0, 1, 0, 1, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 1, 1, 1, 0, 1, 1, 0, 0, 1, 1, 0, 1, 0, 0, 0, 1,
		1, 1, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 1, 0, 1, 1, 0, 0, 0, 1, 1, 0, 0, 0,
	}
	for i := 0; i < len(interleaved); i++ {
		transmission[i] = Symbols[syncWord[i]+2*interleaved[i]]
	}
	return
}
