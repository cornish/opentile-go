package jpeg

import (
	"bytes"
	"fmt"
)

// adobeAPP14 is the 16-byte Adobe APP14 segment Python opentile splices
// before SOS to advertise RGB (not YCbCr) colorspace, matching Aperio's
// non-standard JPEG encoding.
//
// Python opentile emits this exact byte sequence (jpeg/jpeg.py:392-405 in
// opentile 0.20.0):
//
//	b"\xff\xee\x00\x0e\x41\x64\x6f\x62\x65\x00\x64\x80\x00\x00\x00\x00"
//
// Layout:
//
//	FF EE            APP14 marker
//	00 0E            length = 14 (length field + 12 data bytes)
//	41 64 6F 62 65 00  identifier "Adobe\0"
//	64 80            DCTEncodeVersion (Python writes 0x6480)
//	00 00            APP14Flags0
//	00 00            APP14Flags1
//
// Note: this is 12 bytes of Adobe payload; the standard Adobe APP14 segment
// also carries a 1-byte ColorTransform field (total payload 13 bytes,
// length 15). Python opentile omits that byte, so the segment as emitted is
// technically a truncated Adobe APP14. Most JPEG decoders infer RGB from
// the presence of the Adobe identifier alone when ColorTransform is absent,
// and we preserve Python's bytes exactly to satisfy the parity oracle.
var adobeAPP14 = []byte{
	0xFF, 0xEE, 0x00, 0x0E,
	0x41, 0x64, 0x6F, 0x62, 0x65, 0x00, // "Adobe\0"
	0x64, 0x80, // DCTEncodeVersion
	0x00, 0x00, // APP14Flags0
	0x00, 0x00, // APP14Flags1
}

// InsertTablesAndAPP14 returns a copy of frame with the JPEGTables DQT/DHT
// segments and an Adobe APP14 colorspace-fix segment inserted immediately
// before the first SOS marker. This matches Python opentile's
// Jpeg._add_jpeg_tables_and_rgb_color_space_fix helper byte-for-byte
// (jpeg/jpeg.py:391-405 in opentile 0.20.0).
//
// tables is the raw TIFF tag 347 (JPEGTables) value — a complete JPEG with
// SOI at the start and EOI at the end, carrying only DQT/DHT between. The
// SOI and EOI wrappers are stripped (tables[2:-2]) before insertion.
//
// The APP14 segment signals to JPEG decoders that the component data is
// RGB (not YCbCr), matching Aperio's non-standard colorspace encoding.
//
// Used by SVS tiled JPEG to turn abbreviated per-tile TIFF scan bytes into
// standalone valid JPEGs.
func InsertTablesAndAPP14(frame, tables []byte) ([]byte, error) {
	if len(tables) < 4 {
		return nil, fmt.Errorf("%w: JPEGTables too short (%d bytes, want >=4)", ErrBadJPEG, len(tables))
	}
	sosIdx := bytes.Index(frame, []byte{0xFF, byte(SOS)})
	if sosIdx < 0 {
		return nil, fmt.Errorf("%w: SOS marker not found", ErrBadJPEG)
	}
	tablesMid := tables[2 : len(tables)-2] // strip SOI and EOI wrappers

	out := make([]byte, 0, len(frame)+len(tablesMid)+len(adobeAPP14))
	out = append(out, frame[:sosIdx]...)
	out = append(out, tablesMid...)
	out = append(out, adobeAPP14...)
	out = append(out, frame[sosIdx:]...)
	return out, nil
}
