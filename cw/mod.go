package cw

import (
	"errors"
	"fmt"
	"unicode"
)

type Modulator struct {
	symbols chan interface{}
	closed  chan struct{}

	pitchFrequency float64
	wpm            int
	dit            float64
	fwpm           int
	fdit           float64
	window         float64
	symbolStart    float64
	symbolEnd      float64
	keyDown        bool
}

func NewModulator(frequency float64, wpm int) *Modulator {
	dit := WPMToSeconds(wpm)
	return &Modulator{
		symbols:        make(chan any, 100),
		closed:         make(chan struct{}),
		pitchFrequency: frequency,
		wpm:            wpm,
		dit:            dit,
		fwpm:           wpm,
		fdit:           dit,
		window:         7.5 / frequency,
	}
}

var ErrWriteAborted = errors.New("cw: write aborted")

type endOfTransmissionToken chan interface{}

func (m *Modulator) SetFarnsworthWPM(fwpm int) {
	if fwpm == 0 {
		m.ClearFarnsworth()
		return
	}
	m.fwpm = fwpm
	m.fdit = float64(FarnsworthWPMToSeconds(m.wpm, fwpm))
}

func (m *Modulator) ClearFarnsworth() {
	m.fwpm = m.wpm
	m.fdit = m.dit
}

func (m *Modulator) customDit(wpm int) float64 {
	if wpm == 0 {
		return WPMToSeconds(m.wpm)
	}
	return WPMToSeconds(wpm)
}

func (m *Modulator) Close() error {
	select {
	case <-m.closed:
	default:
		close(m.closed)
	}
	return nil
}

func (m *Modulator) AbortWhenDone(done <-chan struct{}) {
	go func() {
		select {
		case <-done:
			m.Close()
		case <-m.closed:
		}
	}()
}

func (m *Modulator) Write(bytes []byte) (int, error) {
	written := 0
	wasWhitespace := true
	canceled := false
	for _, r := range string(bytes) {
		if canceled {
			return written, ErrWriteAborted
		}

		normalized := unicode.ToLower(r)
		if unicode.IsSpace(normalized) {
			if !wasWhitespace {
				canceled = m.writeSymbol(WordBreak)
			}

			if !canceled {
				written++
			}
			wasWhitespace = true
			continue
		}

		code, knownCode := Code[normalized]
		if !knownCode {
			continue
		}
		if !wasWhitespace {
			canceled = m.writeSymbol(CharBreak)
		}
		firstSymbol := true
		for _, s := range code {
			if !firstSymbol {
				canceled = m.writeSymbol(SymbolBreak)
			}
			canceled = m.writeSymbol(s)
			firstSymbol = false
		}

		if !canceled {
			written++
		}
		wasWhitespace = false
	}

	if !wasWhitespace && m.writeSymbol(WordBreak) {
		return written, ErrWriteAborted
	}
	if m.waitForEndOfTransmission() {
		return written, ErrWriteAborted
	}
	return written, nil
}

func (m *Modulator) writeSymbol(symbol Symbol) bool {
	select {
	case m.symbols <- symbol:
		return false
	case <-m.closed:
		return true
	}
}

func (m *Modulator) waitForEndOfTransmission() bool {
	eot := make(endOfTransmissionToken)
	select {
	case m.symbols <- eot:
	case <-m.closed:
		return true
	}
	select {
	case <-eot:
		return false
	case <-m.closed:
		return true
	}
}

func (m *Modulator) Modulate(t, a, f, p float64) (amplitude, frequency, phase float64) {
	var delta float64
	switch {
	case m.symbolEnd-t <= m.window:
		delta = m.symbolEnd - t
	case t-m.symbolStart <= m.window:
		delta = t - m.symbolStart
	default:
		delta = m.window
	}
	if m.keyDown {
		amplitude = delta / m.window
	} else {
		amplitude = 0
	}

	if m.symbolEnd > t {
		return amplitude, m.pitchFrequency, p
	}
	nextEnd, keyDown, canceled := m.nextAction(t)
	if canceled {
		return 0, m.pitchFrequency, p
	}

	m.symbolStart = t
	m.symbolEnd = nextEnd
	m.keyDown = keyDown

	return amplitude, m.pitchFrequency, p
}

func (m *Modulator) nextAction(now float64) (float64, bool, bool) {
	select {
	case raw := <-m.symbols:
		switch symbol := raw.(type) {
		case Symbol:
			duration := m.duration(symbol)
			return now + duration, symbol.KeyDown, false
		case endOfTransmissionToken:
			close(symbol)
			return now + 0.000_01, false, false
		default:
			panic(fmt.Errorf("unknown token/symbol type %T", raw))
		}
	case <-m.closed:
		return now, false, true
	default:
		return now + 0.000_01, false, false
	}
}

func (m *Modulator) duration(symbol Symbol) float64 {
	var dit float64
	switch symbol {
	case CharBreak, WordBreak:
		dit = m.fdit
	default:
		dit = m.dit
	}
	return float64(symbol.Weight) * dit
}
