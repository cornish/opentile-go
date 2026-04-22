package jpeg

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ConcatOpts controls how ConcatenateScans assembles the output JPEG.
//
// Fragment-level APPn segments are discarded during assembly; only DQT/DHT
// from JPEGTables and the caller-supplied ColorspaceFix APP14 are included
// in the output header.
//
// When RestartInterval > 0, ConcatenateScans expects each input fragment's
// scan data to contain exactly one restart interval (no internal RSTn
// markers), because the inter-fragment RST codes are assigned via a global
// cycle 0xD0..0xD7 indexed by fragment position. Fragments with multiple
// internal restart intervals will cause cycle drift at boundaries and a
// malformed output bitstream. NDPI and SVS associated-image stripes are
// both single-interval in practice, which is the intended use case.
//
// When RestartInterval == 0, scan data is concatenated verbatim with no
// separator. This is almost never what a caller wants for multi-fragment
// input: the decoder's DC-coefficient predictor carries state across the
// boundary, producing color drift unless the fragments are a single
// continuous scan. Use 0 only for single-fragment input or when the caller
// has independently ensured predictor continuity.
type ConcatOpts struct {
	Width, Height   uint16 // output SOF dimensions
	JPEGTables      []byte // raw TIFF JPEGTables value; DQT/DHT extracted via SplitJPEGTables
	ColorspaceFix   bool   // if true, emit an APP14 "Adobe" segment signalling RGB (for SVS non-standard RGB JPEGs)
	RestartInterval int    // see godoc above: 0 means no DRI and verbatim concat; >0 means one restart interval per fragment
}

// ConcatenateScans builds a single valid JPEG from one or more TIFF-embedded
// JPEG fragments. Each fragment is a mini-JPEG (SOI ... SOS + scan + EOI);
// the assembled output uses the tables from opts.JPEGTables and its own SOF
// derived from opts.Width/Height, with the scan data of each fragment
// concatenated in order and optionally separated by restart markers.
func ConcatenateScans(fragments [][]byte, opts ConcatOpts) ([]byte, error) {
	if len(fragments) == 0 {
		return nil, fmt.Errorf("%w: no fragments", ErrBadJPEG)
	}

	dqts, dhts, err := SplitJPEGTables(opts.JPEGTables)
	if err != nil {
		return nil, fmt.Errorf("split tables: %w", err)
	}

	// Determine the SOF from the first fragment (for component sampling info)
	// and override width/height from opts.
	var firstSOF *SOF
	var firstSOS []byte
	for seg, err := range Scan(bytes.NewReader(fragments[0])) {
		if err != nil {
			return nil, fmt.Errorf("scan first fragment: %w", err)
		}
		switch seg.Marker {
		case SOF0:
			s, err := ParseSOF(seg.Payload)
			if err != nil {
				return nil, err
			}
			firstSOF = s
		case SOS:
			firstSOS = make([]byte, 0, 4+len(seg.Payload))
			firstSOS = append(firstSOS, 0xFF, byte(SOS))
			length := 2 + len(seg.Payload)
			lb := make([]byte, 2)
			binary.BigEndian.PutUint16(lb, uint16(length))
			firstSOS = append(firstSOS, lb...)
			firstSOS = append(firstSOS, seg.Payload...)
		}
		if firstSOF != nil && firstSOS != nil {
			break
		}
	}
	if firstSOF == nil {
		return nil, fmt.Errorf("%w: first fragment missing SOF", ErrBadJPEG)
	}
	if firstSOS == nil {
		return nil, fmt.Errorf("%w: first fragment missing SOS", ErrBadJPEG)
	}
	sof := &SOF{
		Precision:  firstSOF.Precision,
		Width:      opts.Width,
		Height:     opts.Height,
		Components: firstSOF.Components,
	}

	var out bytes.Buffer
	out.Write([]byte{0xFF, 0xD8}) // SOI
	if opts.ColorspaceFix {
		// Adobe APP14 segment: 16 bytes total (FF EE + length 00 0E + 12 data
		// bytes). The "Adobe" identifier has NO null terminator; the length
		// field counts itself (2) + the 12 data bytes = 14.
		app14 := []byte{
			0xFF, 0xEE, 0x00, 0x0E,
			'A', 'd', 'o', 'b', 'e',
			0x64, 0x00, // DCTEncodeVersion
			0x00, 0x00, // APP14Flags0
			0x00, 0x00, // APP14Flags1
			0x00,       // ColorTransform = 0 (RGB)
		}
		out.Write(app14)
	}
	for _, seg := range dqts {
		out.Write(seg)
	}
	for _, seg := range dhts {
		out.Write(seg)
	}
	out.Write(BuildSOF(sof))
	if opts.RestartInterval > 0 {
		// DRI: marker + length=4 + interval (u16)
		dri := []byte{0xFF, 0xDD, 0x00, 0x04, 0, 0}
		binary.BigEndian.PutUint16(dri[4:], uint16(opts.RestartInterval))
		out.Write(dri)
	}
	out.Write(firstSOS)

	// Concatenate entropy data from each fragment, inserting restart markers
	// between fragments when RestartInterval > 0.
	for i, frag := range fragments {
		scanData, err := extractScanData(frag)
		if err != nil {
			return nil, fmt.Errorf("fragment %d: %w", i, err)
		}
		out.Write(scanData)
		if opts.RestartInterval > 0 && i < len(fragments)-1 {
			rstCode := byte(0xD0 + (i % 8))
			out.Write([]byte{0xFF, rstCode})
		}
	}
	out.Write([]byte{0xFF, 0xD9}) // EOI
	return out.Bytes(), nil
}

// extractScanData walks frag as a sequence of JPEG segments, locates the SOS,
// and returns the entropy-coded scan data that follows it (up to and not
// including the trailing EOI). The walk parses each marker segment's length
// and skips its payload, so it cannot false-match on APPn payload bytes that
// happen to look like marker codes.
func extractScanData(frag []byte) ([]byte, error) {
	scanStart, err := findSOSScanStart(frag)
	if err != nil {
		return nil, err
	}
	data, end, err := ReadScan(bytes.NewReader(frag[scanStart:]))
	if err != nil {
		return nil, err
	}
	if end != EOI {
		return nil, fmt.Errorf("%w: scan ended with 0x%X, want EOI", ErrBadJPEG, end)
	}
	return data, nil
}

// findSOSScanStart walks frag segment-by-segment and returns the byte offset
// of the first byte of entropy-coded scan data (immediately after the SOS
// segment's length-prefixed payload).
func findSOSScanStart(frag []byte) (int, error) {
	// JPEG segments start at offset 0 with SOI (FF D8) or may have fill bytes.
	pos := 0
	for pos < len(frag) {
		// Skip fill bytes and locate the marker code.
		for pos < len(frag) && frag[pos] != 0xFF {
			// A non-FF byte outside segment framing is malformed.
			return 0, fmt.Errorf("%w: unexpected byte 0x%02X at pos %d", ErrBadJPEG, frag[pos], pos)
		}
		for pos < len(frag) && frag[pos] == 0xFF {
			pos++
		}
		if pos >= len(frag) {
			return 0, fmt.Errorf("%w: truncated before marker code", ErrBadJPEG)
		}
		code := Marker(frag[pos])
		pos++
		if code == 0x00 {
			return 0, fmt.Errorf("%w: 0xFF00 outside scan data", ErrBadJPEG)
		}
		if code == SOS {
			// Read 2-byte length, skip payload, return next position.
			if pos+2 > len(frag) {
				return 0, fmt.Errorf("%w: SOS truncated length at pos %d", ErrBadJPEG, pos)
			}
			sosLen := int(binary.BigEndian.Uint16(frag[pos : pos+2]))
			if sosLen < 2 {
				return 0, fmt.Errorf("%w: SOS length %d < 2", ErrBadJPEG, sosLen)
			}
			scanStart := pos + sosLen
			if scanStart > len(frag) {
				return 0, fmt.Errorf("%w: scan start past frag end", ErrBadJPEG)
			}
			return scanStart, nil
		}
		if code.isStandalone() {
			// SOI, EOI, RSTn: no length, no payload. But we shouldn't hit
			// EOI before SOS — that would mean no scan.
			if code == EOI {
				return 0, fmt.Errorf("%w: EOI before SOS", ErrBadJPEG)
			}
			continue
		}
		// Length-prefixed segment: read length, skip payload.
		if pos+2 > len(frag) {
			return 0, fmt.Errorf("%w: truncated length at pos %d", ErrBadJPEG, pos)
		}
		segLen := int(binary.BigEndian.Uint16(frag[pos : pos+2]))
		if segLen < 2 {
			return 0, fmt.Errorf("%w: segment length %d < 2", ErrBadJPEG, segLen)
		}
		pos += segLen
		if pos > len(frag) {
			return 0, fmt.Errorf("%w: segment past frag end", ErrBadJPEG)
		}
	}
	return 0, fmt.Errorf("%w: no SOS found", ErrBadJPEG)
}
