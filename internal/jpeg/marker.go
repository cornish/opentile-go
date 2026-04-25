// Package jpeg provides marker-level JPEG bitstream manipulation sufficient
// to assemble valid JPEGs from TIFF-embedded scan fragments. It does not
// decode or encode pixel data; callers wanting decoded pixels should pass
// this package's output to a JPEG codec of their choice.
//
// The package is deliberately narrow: it understands the 2-byte marker /
// length segment framing of a JPEG bitstream (JFIF JPEG / baseline DCT), can
// round-trip segment sequences, can rewrite the SOF0 dimensions in-place,
// and can concatenate multiple scan fragments (as stored in TIFF tile/stripe
// payloads) into a single valid JPEG by prepending an appropriate header
// derived from the TIFF JPEGTables tag. It does not interpret entropy-coded
// scan data beyond preserving byte stuffing.
package jpeg

import "errors"

// ErrBadJPEG is surfaced when a JPEG bitstream cannot be parsed.
// The top-level opentile package re-exports this as ErrBadJPEGBitstream for
// consumers who import by sentinel rather than by package.
var ErrBadJPEG = errors.New("jpeg: invalid bitstream")

// Marker is the one-byte marker code that follows a 0xFF prefix byte.
type Marker byte

const (
	SOI   Marker = 0xD8 // Start Of Image
	EOI   Marker = 0xD9 // End Of Image
	SOS   Marker = 0xDA // Start Of Scan
	DQT   Marker = 0xDB // Define Quantization Table
	DHT   Marker = 0xC4 // Define Huffman Table
	SOF0  Marker = 0xC0 // Baseline DCT Start Of Frame
	DRI   Marker = 0xDD // Define Restart Interval
	COM   Marker = 0xFE // Comment
	APP0  Marker = 0xE0 // APPn range start
	APP14 Marker = 0xEE
	RST0  Marker = 0xD0 // RST0..RST7 occupy 0xD0..0xD7
)

// Segment is a parsed marker + its payload. Payload excludes the 2-byte
// length prefix when present; it is nil for stand-alone markers (SOI, EOI,
// RSTn). For SOS, Payload holds the scan header parameters; the entropy-
// coded scan that follows is not read by the Scan iterator.
type Segment struct {
	Marker  Marker
	Payload []byte
}

// isStandalone reports whether m is a stand-alone marker (no length / payload).
func (m Marker) isStandalone() bool {
	switch m {
	case SOI, EOI, 0x01, 0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7:
		return true
	}
	return false
}
