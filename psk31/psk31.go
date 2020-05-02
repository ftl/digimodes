/*
Package psk31 implements the PSK31 digital mode.
*/
package psk31

import (
	"errors"
	"fmt"
	"math"
)

const (
	window = 10
	raster = 32

	preambleLength = 25
	endLength      = 25
)

// Symbol for PSK
type Symbol uint16

// Modulator generates a PSK31 signal and provides the io.Writer interface.
type Modulator struct {
	symbols chan interface{}
	packed  chan interface{}
	closed  chan struct{}

	block            block
	blocks           *blocks
	phaseSwitchCycle bool

	carrierFrequency float64
}

type block interface {
	Cycle(a, p, delta float64, phaseSwitchCycle bool) (amplitude, phase float64, needNextBlock bool)
}

func NewModulator(frequency float64) *Modulator {
	result := &Modulator{
		symbols:          make(chan interface{}),
		packed:           make(chan interface{}),
		closed:           make(chan struct{}),
		carrierFrequency: frequency,
		blocks:           newBlocks(),
	}
	result.block = result.blocks.off(false)
	go result.pack()
	return result
}

var ErrWriteAborted = errors.New("psk31: write aborted")

type preambleToken chan interface{}
type endOfTransmissionToken chan interface{}
type endToken chan interface{}

func (m *Modulator) End() error {
	end := make(endToken)
	m.symbols <- end
	select {
	case <-end:
		return nil
	case <-m.closed:
		return ErrWriteAborted
	}
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
	m.symbols <- make(preambleToken)

	n := 0
	for _, b := range bytes {
		select {
		case m.symbols <- Varicode[b&0x7F]:
			n++
		case <-m.closed:
			return n, ErrWriteAborted
		}
	}

	eot := make(endOfTransmissionToken)
	m.symbols <- eot
	select {
	case <-eot:
		return n, nil
	case <-m.closed:
		return n, ErrWriteAborted
	}
}

func (m *Modulator) pack() {
	packer := symbolPacker{}
	for {
		select {
		case s := <-m.symbols:
			packer.Pack(m.packed, s)
		case <-m.closed:
			return
		}
	}
}

type symbolPacker struct {
	out         uint8
	lastWasZero bool
	outBitIndex int
	dirty       bool
}

func (p *symbolPacker) Pack(packed chan<- interface{}, s interface{}) {
	switch in := s.(type) {
	case Symbol:
		p.dirty = true
		for i := 15; i >= 0; i-- {
			inBit := (in >> uint8(i)) & 0x0001
			p.out = (p.out << 1) | uint8(inBit)
			p.outBitIndex = (p.outBitIndex + 1) % 8

			if p.outBitIndex == 0 {
				packed <- p.out
				p.out = 0
			}

			if p.lastWasZero && (inBit == 0) {
				break
			}
			p.lastWasZero = (inBit == 0)
		}
	default: // all the tokens
		p.Flush(packed)
		packed <- in
	}
}

func (p *symbolPacker) Flush(packed chan<- interface{}) {
	if (p.outBitIndex == 0 && p.lastWasZero) || !p.dirty {
		p.dirty = false
		return
	}

	p.out = (p.out << uint8(8-p.outBitIndex))
	packed <- p.out

	if p.out&0x3 != 0 {
		packed <- uint8(0)
	}

	p.out = 0
	p.outBitIndex = 0
	p.dirty = false
}

func (m *Modulator) Modulate(t, a, f, p float64) (amplitude, frequency, phase float64) {
	ms := t * 1000.0
	fraction := ms - float64(int(ms))
	rasterTime := int(ms) % raster

	var delta float64
	switch {
	case rasterTime < window:
		delta = float64(rasterTime) + fraction
	case rasterTime > raster-window:
		delta = float64(raster-rasterTime) - fraction
	default:
		delta = float64(window)
	}

	var needNextBlock bool

	amplitude, phase, needNextBlock = m.block.Cycle(a, p, delta, rasterTime == 0 && m.phaseSwitchCycle)
	m.phaseSwitchCycle = rasterTime != 0

	if needNextBlock {
		m.block = m.blocks.Next(m.packed, m.block, m.closed)
	}

	return amplitude, m.carrierFrequency, phase
}

type blocks struct {
	_off      *offBlock
	_preamble *preambleBlock
	_transmit *transmitBlock
	_end      *endBlock
}

func newBlocks() *blocks {
	return &blocks{
		_off:      new(offBlock),
		_preamble: new(preambleBlock),
		_transmit: new(transmitBlock),
		_end:      new(endBlock),
	}
}

func (b *blocks) Next(packedSymbols <-chan interface{}, currentBlock block, closed <-chan struct{}) block {
	select {
	case s := <-packedSymbols:
		switch s := s.(type) {
		case uint8:
			return b.transmit(s)
		case preambleToken:
			if _, ok := currentBlock.(*transmitBlock); ok {
				close(s)
				return b.Next(packedSymbols, currentBlock, closed)
			}
			return b.preamble(s)
		case endOfTransmissionToken:
			close(s)
			return b.Next(packedSymbols, currentBlock, closed)
		case endToken:
			return b.end(s)
		default:
			panic(fmt.Sprintf("unknown token type %T", s))
		}
	case <-closed:
		return b.off(true)
	default:
		return currentBlock
	}
}

func (b *blocks) off(closed bool) *offBlock {
	b._off.closed = closed
	return b._off
}

func (b *blocks) preamble(token preambleToken) *preambleBlock {
	b._preamble.cycles = preambleLength
	b._preamble.token = token
	return b._preamble
}

func (b *blocks) transmit(bits uint8) *transmitBlock {
	b._transmit.bits = bits
	b._transmit.bitIndex = 0
	b._transmit.finished = false
	return b._transmit
}

func (b *blocks) end(token endToken) *endBlock {
	b._end.cycles = endLength
	b._end.token = token
	return b._end
}

type offBlock struct {
	closed bool
}

func (b *offBlock) Cycle(a, p, delta float64, phaseSwitchCycle bool) (amplitude, phase float64, needNextBlock bool) {
	return 0, 0, !b.closed
}

type preambleBlock struct {
	cycles int
	token  preambleToken
}

func (b *preambleBlock) Cycle(a, p, delta float64, phaseSwitchCycle bool) (amplitude, phase float64, needNextBlock bool) {
	if b.cycles == preambleLength {
		amplitude = a
	} else {
		amplitude = delta / float64(window)
	}
	phase = p
	needNextBlock = false
	if phaseSwitchCycle {
		if p == 0 {
			phase = math.Pi
		} else {
			phase = 0.0
		}
		select {
		case <-b.token:
			needNextBlock = true
		default:
			b.cycles--
			if b.cycles == 0 {
				close(b.token)
				needNextBlock = true
			}
		}
	}
	return amplitude, phase, needNextBlock
}

type transmitBlock struct {
	bits     uint8
	bitIndex uint8
	finished bool
}

func (b *transmitBlock) Cycle(a, p, delta float64, phaseSwitchCycle bool) (amplitude, phase float64, needNextBlock bool) {
	amplitude = delta / float64(window)

	phase = p
	if phaseSwitchCycle {
		var bit uint8
		if !b.finished {
			bit = (b.bits >> uint8(7-b.bitIndex)) & 0x01
			b.bitIndex = (b.bitIndex + 1) % 8
			b.finished = b.bits == 0 || b.bitIndex == 0
		} else {
			bit = 0
		}

		switchPhase := (bit == 0)
		if switchPhase {
			if p == 0 {
				phase = math.Pi
			} else {
				phase = 0.0
			}
		}
	}

	needNextBlock = b.finished

	return amplitude, phase, needNextBlock
}

type endBlock struct {
	cycles int
	token  endToken
}

func (b *endBlock) Cycle(a, p, delta float64, phaseSwitchCycle bool) (amplitude, phase float64, needNextBlock bool) {
	newAmplitude := delta / float64(window)
	switch {
	case b.cycles == endLength && a < newAmplitude:
		amplitude = newAmplitude
	case b.cycles == 1 && a > newAmplitude:
		amplitude = newAmplitude
	default:
		amplitude = a
	}

	needNextBlock = false
	if phaseSwitchCycle {
		select {
		case <-b.token:
			needNextBlock = true
		default:
			b.cycles--
			if b.cycles == 0 {
				close(b.token)
				needNextBlock = true
			}
		}
	}
	return amplitude, p, needNextBlock
}
