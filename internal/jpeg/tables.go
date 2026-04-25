package jpeg

import (
	"encoding/binary"
	"fmt"
)

// SplitJPEGTables parses the value of a TIFF JPEGTables tag and returns each
// DQT and DHT segment separately. Each returned slice element is the full
// segment bytes including the 0xFF marker prefix and 2-byte length, ready
// to concatenate into a new bitstream.
//
// JPEGTables is a mini-JPEG containing SOI, one or more DQT, one or more
// DHT, and EOI. Other segments (COM, APPn) are tolerated but ignored.
func SplitJPEGTables(tables []byte) (dqts [][]byte, dhts [][]byte, err error) {
	if len(tables) < 4 || tables[0] != 0xFF || Marker(tables[1]) != SOI {
		return nil, nil, fmt.Errorf("%w: JPEGTables does not start with SOI", ErrBadJPEG)
	}
	pos := 2
	for pos < len(tables) {
		// Skip any fill bytes first.
		for pos < len(tables) && tables[pos] == 0xFF {
			pos++
		}
		if pos >= len(tables) {
			return nil, nil, fmt.Errorf("%w: truncated after fill bytes", ErrBadJPEG)
		}
		code := Marker(tables[pos])
		pos++
		if code == EOI {
			return dqts, dhts, nil
		}
		if code.isStandalone() {
			continue
		}
		if pos+2 > len(tables) {
			return nil, nil, fmt.Errorf("%w: truncated length at pos %d", ErrBadJPEG, pos)
		}
		length := int(binary.BigEndian.Uint16(tables[pos : pos+2]))
		if length < 2 {
			return nil, nil, fmt.Errorf("%w: segment length %d < 2", ErrBadJPEG, length)
		}
		end := pos + length
		if end > len(tables) {
			return nil, nil, fmt.Errorf("%w: segment extends past buffer (end=%d, len=%d)", ErrBadJPEG, end, len(tables))
		}
		seg := make([]byte, 0, 2+length)
		seg = append(seg, 0xFF, byte(code))
		seg = append(seg, tables[pos:end]...)
		pos = end
		switch code {
		case DQT:
			dqts = append(dqts, seg)
		case DHT:
			dhts = append(dhts, seg)
		}
	}
	return dqts, dhts, nil
}
