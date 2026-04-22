package jpeg

import (
	"encoding/binary"
	"fmt"
)

// SOF describes a Start-Of-Frame-0 (baseline DCT) segment's parameters.
type SOF struct {
	Precision     uint8 // typically 8 for baseline DCT
	Height, Width uint16
	Components    []SOFComponent
}

// SOFComponent describes one component within an SOF segment.
type SOFComponent struct {
	ID           uint8 // 1=Y, 2=Cb, 3=Cr for YCbCr; 1=R, 2=G, 3=B for RGB
	SamplingH    uint8
	SamplingV    uint8
	QuantTableID uint8
}

// MCUSize returns the minimum coded unit size in pixels, derived from the
// maximum horizontal and vertical sampling factors across components.
// For YCbCr 4:2:0 (Y=2,2 others 1,1) → 16x16; 4:4:4 → 8x8; 4:2:2 → 16x8.
func (s *SOF) MCUSize() (w, h int) {
	var maxH, maxV uint8 = 1, 1
	for _, c := range s.Components {
		if c.SamplingH > maxH {
			maxH = c.SamplingH
		}
		if c.SamplingV > maxV {
			maxV = c.SamplingV
		}
	}
	return int(maxH) * 8, int(maxV) * 8
}

// ParseSOF decodes a SOF0 segment payload (the bytes AFTER the 2-byte length).
func ParseSOF(payload []byte) (*SOF, error) {
	if len(payload) < 6 {
		return nil, fmt.Errorf("%w: SOF payload %d < 6", ErrBadJPEG, len(payload))
	}
	s := &SOF{
		Precision: payload[0],
		Height:    binary.BigEndian.Uint16(payload[1:3]),
		Width:     binary.BigEndian.Uint16(payload[3:5]),
	}
	n := int(payload[5])
	expected := 6 + 3*n
	if len(payload) < expected {
		return nil, fmt.Errorf("%w: SOF payload %d < needed %d", ErrBadJPEG, len(payload), expected)
	}
	s.Components = make([]SOFComponent, n)
	for i := 0; i < n; i++ {
		off := 6 + 3*i
		samp := payload[off+1]
		s.Components[i] = SOFComponent{
			ID:           payload[off],
			SamplingH:    samp >> 4,
			SamplingV:    samp & 0x0F,
			QuantTableID: payload[off+2],
		}
	}
	return s, nil
}

// BuildSOF encodes an SOF struct as a complete marker segment (prefix
// 0xFF 0xC0, 2-byte length, payload). The returned slice is ready to
// concatenate into a new bitstream.
func BuildSOF(s *SOF) []byte {
	n := len(s.Components)
	length := 2 + 6 + 3*n
	out := make([]byte, 2+length)
	out[0] = 0xFF
	out[1] = byte(SOF0)
	binary.BigEndian.PutUint16(out[2:4], uint16(length))
	out[4] = s.Precision
	binary.BigEndian.PutUint16(out[5:7], s.Height)
	binary.BigEndian.PutUint16(out[7:9], s.Width)
	out[9] = byte(n)
	for i, c := range s.Components {
		o := 10 + 3*i
		out[o] = c.ID
		out[o+1] = (c.SamplingH << 4) | (c.SamplingV & 0x0F)
		out[o+2] = c.QuantTableID
	}
	return out
}
