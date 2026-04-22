package jpeg

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"iter"
)

// Scan returns a Go 1.23 iterator over JPEG segments. The iterator stops
// after yielding the EOI marker. It does not follow the entropy-coded scan
// data after SOS — callers that need the scan bytes use ReadScan on the
// reader after the SOS Segment is yielded.
//
// Malformed inputs yield a Segment with a zero-valued Marker and a non-nil
// error. Callers must honor the error yield and stop iterating.
func Scan(r io.Reader) iter.Seq2[Segment, error] {
	return func(yield func(Segment, error) bool) {
		br := bufio.NewReader(r)
		for {
			// Every marker begins with 0xFF, possibly preceded by any number
			// of 0xFF fill bytes.
			b, err := br.ReadByte()
			if err != nil {
				yield(Segment{}, fmt.Errorf("%w: read marker prefix: %v", ErrBadJPEG, err))
				return
			}
			if b != 0xFF {
				yield(Segment{}, fmt.Errorf("%w: expected 0xFF, got 0x%02X", ErrBadJPEG, b))
				return
			}
			// Skip fill bytes: consecutive 0xFF until a non-0xFF code.
			var code byte
			for {
				code, err = br.ReadByte()
				if err != nil {
					yield(Segment{}, fmt.Errorf("%w: read marker code: %v", ErrBadJPEG, err))
					return
				}
				if code != 0xFF {
					break
				}
			}
			if code == 0x00 {
				yield(Segment{}, fmt.Errorf("%w: 0xFF00 stuffed byte outside scan data", ErrBadJPEG))
				return
			}
			m := Marker(code)
			if m.isStandalone() {
				if !yield(Segment{Marker: m}, nil) {
					return
				}
				if m == EOI {
					return
				}
				continue
			}
			// Marker-segment with 2-byte length.
			var lenBuf [2]byte
			if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
				yield(Segment{}, fmt.Errorf("%w: read length: %v", ErrBadJPEG, err))
				return
			}
			length := binary.BigEndian.Uint16(lenBuf[:])
			if length < 2 {
				yield(Segment{}, fmt.Errorf("%w: segment length %d < 2", ErrBadJPEG, length))
				return
			}
			payload := make([]byte, length-2)
			if _, err := io.ReadFull(br, payload); err != nil {
				yield(Segment{}, fmt.Errorf("%w: read payload: %v", ErrBadJPEG, err))
				return
			}
			if !yield(Segment{Marker: m, Payload: payload}, nil) {
				return
			}
		}
	}
}
