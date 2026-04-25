package jpeg

import (
	"bufio"
	"fmt"
	"io"
)

// ReadScan reads entropy-coded JPEG scan data from r up to (but not
// including) the next non-RST marker. Byte stuffing (0xFF 0x00) is
// preserved in the returned slice because the caller may want to
// concatenate this scan into a new bitstream without decoding.
//
// Returns the scan bytes, the marker code that terminated the scan (with
// the 0xFF prefix stripped), and any read error. If the caller needs the
// stream position to continue past the returned marker, they must then
// consume the marker's length+payload (for length-bearing markers) before
// the next operation. For RST markers (stand-alone) there is nothing more
// to consume.
//
// If r is a *bufio.Reader, ReadScan uses it directly; otherwise it wraps r.
func ReadScan(r io.Reader) (data []byte, end Marker, err error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	var out []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return nil, 0, fmt.Errorf("%w: read scan byte: %v", ErrBadJPEG, err)
		}
		if b != 0xFF {
			out = append(out, b)
			continue
		}
		// Peek the next byte to disambiguate.
		next, err := br.ReadByte()
		if err != nil {
			return nil, 0, fmt.Errorf("%w: read marker after 0xFF: %v", ErrBadJPEG, err)
		}
		switch {
		case next == 0x00:
			// Byte stuffing: represents a literal 0xFF within scan data.
			out = append(out, 0xFF, 0x00)
		case next >= 0xD0 && next <= 0xD7:
			// RSTn — part of the scan, keep going.
			out = append(out, 0xFF, next)
		default:
			// Non-scan marker: end of scan.
			return out, Marker(next), nil
		}
	}
}
