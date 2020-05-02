/*
Package cw implements the CW mode.
*/
package cw

import (
	"context"
	"strings"
	"time"
	"unicode"
)

// WPMToSeconds returns the duration of a dit in seconds with the given speed in WpM.
func WPMToSeconds(wpm int) float64 {
	return (float64(60) / float64(50*wpm))
}

// BPMToSeconds returns the duration of a dit in seconds with the given speed in BpM.
func BPMToSeconds(bpm int) float64 {
	return WPMToSeconds(bpm * 5)
}

// WPMToDit returns the duration of a dit with the given speed in WpM.
func WPMToDit(wpm int) time.Duration {
	return time.Duration(WPMToSeconds(wpm) * float64(time.Second))
}

// BPMToDit returns the duration of a dit with the given speed in BpM.
func BPMToDit(bpm int) time.Duration {
	return WPMToDit(bpm * 5)
}

// Symbol represents the morse symbols: dits, das and breaks.
type Symbol struct {
	Weight  int
	KeyDown bool
}

// All symbols
var (
	Dit         = Symbol{1, true}
	Da          = Symbol{3, true}
	SymbolBreak = Symbol{1, false}
	CharBreak   = Symbol{3, false}
	WordBreak   = Symbol{7, false}
)

// Code contains the morse code table.
var Code = map[rune][]Symbol{
	// characters
	'a': {Dit, Da},
	'b': {Da, Dit, Dit, Dit},
	'c': {Da, Dit, Da, Dit},
	'd': {Da, Dit, Dit},
	'e': {Dit},
	'f': {Dit, Dit, Da, Dit},
	'g': {Da, Da, Dit},
	'h': {Dit, Dit, Dit, Dit},
	'i': {Dit, Dit},
	'j': {Dit, Da, Da, Da},
	'k': {Da, Dit, Da},
	'l': {Dit, Da, Dit, Dit},
	'm': {Da, Da},
	'n': {Da, Dit},
	'o': {Da, Da, Da},
	'p': {Dit, Da, Da, Dit},
	'q': {Da, Da, Dit, Da},
	'r': {Dit, Da, Dit},
	's': {Dit, Dit, Dit},
	't': {Da},
	'u': {Dit, Dit, Da},
	'v': {Dit, Dit, Dit, Da},
	'w': {Dit, Da, Da},
	'x': {Da, Dit, Dit, Da},
	'y': {Da, Dit, Da, Da},
	'z': {Da, Da, Dit, Dit},

	// diacritics
	'ä': {Dit, Da, Dit, Da},
	'ö': {Da, Da, Da, Dit},
	'ü': {Dit, Dit, Da, Da},

	// numbers
	'0': {Da, Da, Da, Da, Da},
	'1': {Dit, Da, Da, Da, Da},
	'2': {Dit, Dit, Da, Da, Da},
	'3': {Dit, Dit, Dit, Da, Da},
	'4': {Dit, Dit, Dit, Dit, Da},
	'5': {Dit, Dit, Dit, Dit, Dit},
	'6': {Da, Dit, Dit, Dit, Dit},
	'7': {Da, Da, Dit, Dit, Dit},
	'8': {Da, Da, Da, Dit, Dit},
	'9': {Da, Da, Da, Da, Dit},

	// punctuation
	'+':  {Dit, Da, Dit, Da, Dit},
	'-':  {Da, Dit, Dit, Dit, Dit, Da},
	'=':  {Da, Dit, Dit, Dit, Da},
	'.':  {Dit, Da, Dit, Da, Dit, Da},
	':':  {Da, Da, Da, Dit, Dit, Dit},
	',':  {Da, Da, Dit, Dit, Da, Da},
	';':  {Da, Dit, Da, Dit, Da, Dit},
	'?':  {Dit, Dit, Da, Da, Dit, Dit},
	'\'': {Dit, Da, Da, Da, Da, Dit},
	'"':  {Dit, Da, Dit, Dit, Da, Dit},
	'(':  {Da, Dit, Da, Da, Dit},
	')':  {Da, Dit, Da, Da, Dit, Da},
	'_':  {Dit, Dit, Da, Da, Dit, Da},
	'@':  {Dit, Da, Da, Dit, Da, Dit},

	// specials
	'[': {Da, Dit, Da, Dit, Da},                   // "Spruchanfang"
	']': {Dit, Dit, Dit, Da, Dit, Da},             // transmission end, "slient key"
	'%': {Dit, Dit, Dit, Da, Dit},                 // understood, "seen"
	'~': {Dit, Da, Dit, Dit, Dit},                 // wait
	'§': {Dit, Dit, Dit, Dit, Dit, Dit, Dit, Dit}, // correction
}

// WriteToSymbolStream writes the content of the given text as morse symbols to the given stream.
// The first written symbol is always a Dit or a Da (key down), the last written symbol is always a WordBreak (key up).
func WriteToSymbolStream(ctx context.Context, symbols chan<- Symbol, text string) {
	normalized := strings.ToLower(text)
	wasWhitespace := true
	var canceled bool
	for _, r := range normalized {
		if canceled {
			return
		}
		if unicode.IsSpace(r) {
			if !wasWhitespace {
				canceled = writeSymbol(ctx, symbols, WordBreak)
			}
			wasWhitespace = true
			continue
		}

		code, knownCode := Code[r]
		if !knownCode {
			continue
		}
		if !wasWhitespace {
			canceled = writeSymbol(ctx, symbols, CharBreak)
		}
		firstSymbol := true
		for _, s := range code {
			if !firstSymbol {
				canceled = writeSymbol(ctx, symbols, SymbolBreak)
			}
			canceled = writeSymbol(ctx, symbols, s)
			firstSymbol = false
		}

		wasWhitespace = false
	}
	if !wasWhitespace {
		canceled = writeSymbol(ctx, symbols, WordBreak)
	}
}

func writeSymbol(ctx context.Context, symbols chan<- Symbol, symbol Symbol) bool {
	select {
	case symbols <- symbol:
		return false
	case <-ctx.Done():
		return true
	}
}

// Send reads CW symbols from the given stream and transmits them using the given setKeyDown function with the given speed in WpM.
func Send(ctx context.Context, setKeyDown func(bool), symbols <-chan Symbol, wpm int) {
	dit := WPMToDit(wpm)

	symbolEnd := time.Now().Add(-1 * time.Second)
	keyDown := false
	canceled := false

	for {
		select {
		case now := <-time.After(1 * time.Microsecond):
			if now.Before(symbolEnd) {
				continue
			}

			symbolEnd, keyDown, canceled = decodeSymbol(ctx, symbols, dit)
			if canceled {
				setKeyDown(false)
				return
			}
			setKeyDown(keyDown)
		case <-ctx.Done():
			setKeyDown(false)
			return
		}
	}
}

func decodeSymbol(ctx context.Context, symbols <-chan Symbol, dit time.Duration) (time.Time, bool, bool) {
	select {
	case symbol := <-symbols:
		duration := time.Duration(symbol.Weight) * dit
		end := time.Now().Add(duration)
		keyDown := symbol.KeyDown
		return end, keyDown, false
	case <-ctx.Done():
		return time.Now(), false, true
	}
}
