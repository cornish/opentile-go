package jpeg

import (
	"bytes"
	"fmt"
)

// adobeAPP14 is the canonical Adobe APP14 segment that Aperio's JPEG encoder
// and Photoshop both emit. Our SVS input slides carry it on the TIFF tile
// bytes are abbreviated (no APP14), so we splice it in before SOS.
//
//	FF EE            APP14 marker
//	00 0E            length = 14 (length field + 12-byte payload)
//	41 64 6F 62 65   identifier "Adobe" (5 bytes, no null terminator)
//	00 64            DCTEncodeVersion = 100
//	80 00            APP14Flags0 = 0x8000
//	00 00            APP14Flags1 = 0
//	00               ColorTransform = 0 (RGB)
//
// ColorTransform = 0 tells decoders the component data is RGB, not YCbCr —
// the "colorspace fix" Aperio needs. This is the same byte sequence Python
// opentile 0.20.0 emits (jpeg/jpeg.py:392-405), preserved exactly for parity.
//
// Single source of truth: both InsertTablesAndAPP14 (SVS tiled) and
// ConcatenateScans (SVS associated images) read from this var.
var adobeAPP14 = []byte{
	0xFF, 0xEE, 0x00, 0x0E,
	0x41, 0x64, 0x6F, 0x62, 0x65, // "Adobe" (5 bytes, no null)
	0x00, 0x64, // DCTEncodeVersion = 100
	0x80, 0x00, // APP14Flags0 = 0x8000
	0x00, 0x00, // APP14Flags1 = 0
	0x00, // ColorTransform = 0 (RGB)
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

// InsertTables returns a copy of frame with the JPEGTables DQT/DHT
// segments inserted immediately before the first SOS marker. Unlike
// InsertTablesAndAPP14, it does not splice an Adobe APP14 marker —
// matches Python opentile's `Jpeg._add_jpeg_tables` helper byte-for-byte
// (jpeg/jpeg.py:421-430).
//
// Used by Philips TIFF tiles, which encode standard YCbCr (no
// colorspace fix needed) but still require the per-page JPEGTables to
// be spliced before the abbreviated TIFF scan bytes can decode.
func InsertTables(frame, tables []byte) ([]byte, error) {
	if len(tables) < 4 {
		return nil, fmt.Errorf("%w: JPEGTables too short (%d bytes, want >=4)", ErrBadJPEG, len(tables))
	}
	sosIdx := bytes.Index(frame, []byte{0xFF, byte(SOS)})
	if sosIdx < 0 {
		return nil, fmt.Errorf("%w: SOS marker not found", ErrBadJPEG)
	}
	tablesMid := tables[2 : len(tables)-2]

	out := make([]byte, 0, len(frame)+len(tablesMid))
	out = append(out, frame[:sosIdx]...)
	out = append(out, tablesMid...)
	out = append(out, frame[sosIdx:]...)
	return out, nil
}

// BuildSplicePrefix returns the constant-per-level splice payload —
// the bytes that go between SOI and the first SOS marker on every
// tile of a given level. Equal to `tables[2:-2]` (DQT/DHT segments
// stripped of the JPEGTables wrapper SOI/EOI), optionally followed
// by the Adobe APP14 colorspace-fix segment for SVS-style RGB JPEG.
//
// Cache the returned []byte on the level struct at construction time
// and pass it to [InsertPrefixInPlace] on each tile read. Same byte
// output as [InsertTablesAndAPP14] / [InsertTables], no per-call alloc.
func BuildSplicePrefix(tables []byte, includeAPP14 bool) ([]byte, error) {
	if len(tables) < 4 {
		return nil, fmt.Errorf("%w: JPEGTables too short (%d bytes, want >=4)", ErrBadJPEG, len(tables))
	}
	tablesMid := tables[2 : len(tables)-2]
	prefix := make([]byte, 0, len(tablesMid)+len(adobeAPP14))
	prefix = append(prefix, tablesMid...)
	if includeAPP14 {
		prefix = append(prefix, adobeAPP14...)
	}
	return prefix, nil
}

// InsertPrefixInPlace splices prefix into a tile that has already been
// read into dst at offset len(prefix), producing the final layout
// (tile_pre_SOS + prefix + SOS_through_EOI) at dst[0:frameLen+len(prefix)].
//
// Caller contract:
//
//   - dst must be at least frameLen + len(prefix) bytes.
//   - dst[len(prefix):len(prefix)+frameLen] holds the original tile
//     bytes at function entry.
//   - On success, dst[0:n] (where n = frameLen + len(prefix)) holds
//     the spliced output, byte-identical to what [InsertTablesAndAPP14]
//     or [InsertTables] would have produced.
//
// Algorithm: find SOS in the read region; in-place memmove the pre-SOS
// bytes backward to the start; memcpy prefix into the gap. The
// SOS-through-EOI portion is already in the right place (the byte
// region dst[len(prefix)+sosIdx:len(prefix)+frameLen] is bit-equal to
// dst[sosIdx+len(prefix):sosIdx+len(prefix)+(frameLen-sosIdx)]) so it
// doesn't need to move.
//
// Zero internal allocations; the work is bounded by frameLen bytes
// of memory traffic (the ReadAt the caller already did) plus a
// sosIdx-bounded backward shift (typically <20 bytes — SOI + maybe
// JFIF/APP0) and a len(prefix)-bounded forward write (DQT/DHT,
// typically a few hundred bytes).
func InsertPrefixInPlace(dst []byte, frameLen int, prefix []byte) (int, error) {
	prefixLen := len(prefix)
	if frameLen < 0 || frameLen+prefixLen > len(dst) {
		return 0, fmt.Errorf("%w: dst too small (need %d, have %d)", ErrBadJPEG, frameLen+prefixLen, len(dst))
	}
	frame := dst[prefixLen : prefixLen+frameLen]
	sosIdx := bytes.Index(frame, []byte{0xFF, byte(SOS)})
	if sosIdx < 0 {
		return 0, fmt.Errorf("%w: SOS marker not found", ErrBadJPEG)
	}
	// Shift pre-SOS bytes from dst[prefixLen:prefixLen+sosIdx] backward
	// to dst[0:sosIdx]. Go's copy handles overlapping slices.
	copy(dst[0:sosIdx], dst[prefixLen:prefixLen+sosIdx])
	// Insert prefix into the gap at dst[sosIdx:sosIdx+prefixLen].
	copy(dst[sosIdx:sosIdx+prefixLen], prefix)
	// dst[sosIdx+prefixLen:sosIdx+prefixLen+(frameLen-sosIdx)] already
	// holds SOS+scan+EOI (same byte region as
	// dst[prefixLen+sosIdx:prefixLen+frameLen]). No copy needed.
	return frameLen + prefixLen, nil
}
