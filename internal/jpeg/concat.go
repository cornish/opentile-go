package jpeg

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ConcatOpts controls how ConcatenateScans assembles the output JPEG.
//
// The output is constructed by appending each input fragment's bytes in
// order (the first whole, subsequent ones starting after their SOS
// header), then patching the in-place SOF size to the accumulated
// dimensions and splicing the JPEGTables' DQT/DHT (plus an optional Adobe
// APP14 marker) immediately before the first SOS. This mirrors Python
// opentile's Jpeg.concatenate_scans byte-for-byte on SVS-style input.
//
// Semantics:
//
//   - Width/Height are the accumulated image dimensions written into the
//     SOF. When they are zero, the dimensions are derived from the input
//     (first fragment width; sum of all fragment heights) to match
//     Python's default behavior. Callers who know the final dimensions
//     (e.g. SVS associated images where the TIFF ImageLength is
//     authoritative) may set them explicitly; the value is the exact
//     uint16 pair written into the output SOF.
//
//   - JPEGTables is the raw TIFF JPEGTables tag value (a mini-JPEG of
//     "SOI DQT DHT EOI"). Its inner bytes (tables[2:-2]) are spliced
//     before the first SOS. If JPEGTables is nil or empty, no splice
//     happens — callers are expected to pass non-empty tables when
//     using ConcatenateScans.
//
//   - ColorspaceFix: when true, emit an Adobe APP14 segment signaling
//     RGB colorspace (ColorTransform=0) immediately after the inserted
//     tables. Required for Aperio SVS non-standard RGB JPEGs. The exact
//     bytes come from the shared adobeAPP14 literal in insert_tables.go.
//
//   - RestartInterval: when >0, either (a) update the existing DRI
//     payload if one is present in the fragment header, or (b) insert a
//     new DRI marker immediately before the first SOS (after the
//     inserted tables/APP14). Python opentile always sets this from
//     "MCUs per scan" so RSTn markers inserted between fragments line
//     up with the decoder's expectation. When 0, no DRI is emitted; in
//     that case RSTn markers are still inserted between fragments
//     (matching Python's behavior), which is correct for inputs whose
//     scan boundary happens to coincide with the natural MCU row
//     boundary.
//
// Fragment-level APPn segments are left untouched in the first fragment
// and dropped from subsequent fragments (which contribute only their
// entropy-coded scan data plus trailing EOI marker). This matches the
// upstream Python behavior of appending `scan[scan_start:]` where
// `scan_start` is past the first-SOS header.
type ConcatOpts struct {
	Width, Height   uint16 // output SOF dimensions; 0 means "derive from input"
	JPEGTables      []byte // raw TIFF JPEGTables value, inserted as tables[2:-2]
	ColorspaceFix   bool   // if true, splice adobeAPP14 after the inserted tables
	RestartInterval int    // 0 = no DRI; >0 = DRI payload value
}

// ConcatenateScans produces a JPEG byte stream byte-for-byte identical to
// Python opentile's Jpeg.concatenate_scans for the same inputs.
//
// Direct port of opentile/jpeg/jpeg.py:concatenate_scans (opentile 0.20.0).
// The algorithm:
//
//  1. Accumulate the frame by appending each fragment in order: the first
//     fragment whole (SOI..SOS..scan..EOI), each subsequent one from its
//     post-SOS scan data through its trailing EOI. Between any two
//     adjacent fragments, rewrite the trailing EOI (FF D9) of the
//     just-appended fragment into FF RSTn (restart_mark(i)).
//
//  2. If JPEGTables is non-empty, splice tables[2:-2] (and optionally
//     adobeAPP14) into the frame immediately before the first SOS.
//
//  3. Patch the SOF dimensions to match the accumulated image size
//     (Width = first-fragment width, Height = sum of fragment heights).
//     If the caller specifies non-zero Width/Height in opts, those
//     override the computed defaults.
//
//  4. If RestartInterval > 0, update an existing DRI in place or insert
//     a new DRI (FF DD 00 04 <interval>) immediately before the first
//     SOS (after the inserted tables/APP14 block).
//
// SOF location: Python's _manipulate_header uses bytes.find(FF C0), a
// naive byte-scan that is theoretically fragile (FF C0 could appear
// inside DQT/DHT payload bytes). For safety we use the jpeg.Scan segment
// iterator to find the real SOF offset in the first fragment, then patch
// bytes in place at that offset. The resulting output is byte-identical
// to Python's on every slide where Python's byte-scan finds the correct
// SOF — which is every slide we've observed in practice. A tables[2:-2]
// blob containing an unstuffed FF C0 would cause our SOF patch to land
// before the inserted tables while Python's would land after; this
// divergence has not been seen on real SVS fixtures.
//
// SOS / DRI location: the only raw FF DA / FF DD markers in the frame
// are the real SOS / DRI markers, because any 0xFF in entropy data is
// byte-stuffed (0xFF 0x00), and the tables blob for our SVS fixtures
// does not contain these marker pairs. We use naive bytes.Index for
// these positions, matching Python's helpers one-to-one.
func ConcatenateScans(fragments [][]byte, opts ConcatOpts) ([]byte, error) {
	if len(fragments) == 0 {
		return nil, fmt.Errorf("%w: no fragments", ErrBadJPEG)
	}

	// --- Step 1: accumulate the frame and track image size. -----------------

	// Parse the first fragment's SOF to seed width/height.
	firstSOF, err := firstFragmentSOF(fragments[0])
	if err != nil {
		return nil, fmt.Errorf("first fragment SOF: %w", err)
	}
	accumW := firstSOF.Width
	accumH := firstSOF.Height

	// Start with the first fragment verbatim.
	frame := make([]byte, 0, initialCap(fragments))
	frame = append(frame, fragments[0]...)

	for i := 1; i < len(fragments); i++ {
		// Between-fragment: rewrite the trailing EOI of the previous
		// fragment (currently at the end of frame) into FF RSTn. Python
		// does this at the END of iteration i-1; we do it at the START
		// of iteration i for exactly the same effect.
		if len(frame) < 2 || frame[len(frame)-2] != 0xFF || frame[len(frame)-1] != 0xD9 {
			return nil, fmt.Errorf("%w: fragment %d: expected trailing EOI before appending next fragment, got %02X %02X",
				ErrBadJPEG, i-1, frame[len(frame)-2], frame[len(frame)-1])
		}
		// restart_mark(i-1) = 0xD0 + ((i-1) % 8).
		frame[len(frame)-2] = 0xFF
		frame[len(frame)-1] = byte(0xD0 + ((i - 1) % 8))

		frag := fragments[i]
		sofI, err := firstFragmentSOF(frag)
		if err != nil {
			return nil, fmt.Errorf("fragment %d SOF: %w", i, err)
		}
		accumH += sofI.Height // widths are expected equal across fragments

		// Find SOS in this fragment and append from scan_start onwards.
		sosPos := bytes.Index(frag, []byte{0xFF, 0xDA})
		if sosPos < 0 {
			return nil, fmt.Errorf("%w: fragment %d: SOS not found", ErrBadJPEG, i)
		}
		if sosPos+4 > len(frag) {
			return nil, fmt.Errorf("%w: fragment %d: SOS truncated length", ErrBadJPEG, i)
		}
		sosLen := int(binary.BigEndian.Uint16(frag[sosPos+2 : sosPos+4]))
		scanStart := sosPos + 2 + sosLen
		if scanStart > len(frag) {
			return nil, fmt.Errorf("%w: fragment %d: scan_start past end", ErrBadJPEG, i)
		}
		frame = append(frame, frag[scanStart:]...)
	}

	// Sanity: the frame should end with the LAST fragment's EOI (0xFF 0xD9).
	if len(frame) < 2 || frame[len(frame)-2] != 0xFF || frame[len(frame)-1] != 0xD9 {
		return nil, fmt.Errorf("%w: assembled frame does not end with EOI", ErrBadJPEG)
	}

	// --- Step 2: splice tables[2:-2] (+ APP14) before first SOS. ------------

	if len(opts.JPEGTables) > 4 {
		insert := opts.JPEGTables[2 : len(opts.JPEGTables)-2]
		if opts.ColorspaceFix {
			combined := make([]byte, 0, len(insert)+len(adobeAPP14))
			combined = append(combined, insert...)
			combined = append(combined, adobeAPP14...)
			insert = combined
		}
		sosPos := bytes.Index(frame, []byte{0xFF, 0xDA})
		if sosPos < 0 {
			return nil, fmt.Errorf("%w: SOS not found in assembled frame", ErrBadJPEG)
		}
		spliced := make([]byte, 0, len(frame)+len(insert))
		spliced = append(spliced, frame[:sosPos]...)
		spliced = append(spliced, insert...)
		spliced = append(spliced, frame[sosPos:]...)
		frame = spliced
	} else if opts.ColorspaceFix {
		// ColorspaceFix without tables is a caller bug for our use cases —
		// if they're asking for the Adobe marker, there must be tables to
		// splice it after. Match Python's layering (APP14 goes with the
		// tables insert); refuse rather than silently producing a frame
		// that doesn't match upstream.
		return nil, fmt.Errorf("%w: ColorspaceFix requires non-empty JPEGTables", ErrBadJPEG)
	}

	// --- Step 3: patch SOF dimensions. --------------------------------------

	finalW := opts.Width
	if finalW == 0 {
		finalW = accumW
	}
	finalH := opts.Height
	if finalH == 0 {
		finalH = accumH
	}
	sofOff, err := findFirstSOFOffset(frame)
	if err != nil {
		return nil, fmt.Errorf("locate SOF for size patch: %w", err)
	}
	// SOF payload starts at sofOff+4 (marker 2 + length 2). Height at
	// payload+1, width at payload+3.
	payload := sofOff + 4
	if payload+5 > len(frame) {
		return nil, fmt.Errorf("%w: SOF payload truncated", ErrBadJPEG)
	}
	binary.BigEndian.PutUint16(frame[payload+1:payload+3], finalH)
	binary.BigEndian.PutUint16(frame[payload+3:payload+5], finalW)

	// --- Step 4: update or insert DRI just before the first SOS. ------------

	if opts.RestartInterval > 0 {
		if opts.RestartInterval > 0xFFFF {
			return nil, fmt.Errorf("%w: RestartInterval %d exceeds uint16", ErrBadJPEG, opts.RestartInterval)
		}
		driPayload := []byte{0, 0}
		binary.BigEndian.PutUint16(driPayload, uint16(opts.RestartInterval))
		driPos := bytes.Index(frame, []byte{0xFF, 0xDD})
		if driPos >= 0 {
			if driPos+6 > len(frame) {
				return nil, fmt.Errorf("%w: DRI truncated", ErrBadJPEG)
			}
			// payload is at driPos+4..driPos+6.
			copy(frame[driPos+4:driPos+6], driPayload)
		} else {
			sosPos := bytes.Index(frame, []byte{0xFF, 0xDA})
			if sosPos < 0 {
				return nil, fmt.Errorf("%w: SOS not found for DRI insert", ErrBadJPEG)
			}
			// Insert FF DD 00 04 <payload> before SOS.
			dri := []byte{0xFF, 0xDD, 0x00, 0x04, driPayload[0], driPayload[1]}
			spliced := make([]byte, 0, len(frame)+len(dri))
			spliced = append(spliced, frame[:sosPos]...)
			spliced = append(spliced, dri...)
			spliced = append(spliced, frame[sosPos:]...)
			frame = spliced
		}
	}

	return frame, nil
}

// FirstFragmentSOF is the exported helper that parses the first SOF0 in a
// JPEG byte buffer. Used by format-package callers (e.g. SVS associated
// images) that need to derive the MCU size and dimensions from a strip
// before assembling a frame.
func FirstFragmentSOF(frag []byte) (*SOF, error) {
	return firstFragmentSOF(frag)
}

// firstFragmentSOF walks seg-by-seg looking for the first SOF0; returns a
// parsed SOF. This uses the segment walker rather than a naive bytes.Index
// so DQT payloads that happen to contain FF C0 cannot fool us — important
// for NDPI-style assemblies that embed larger tables in each fragment.
func firstFragmentSOF(frag []byte) (*SOF, error) {
	for seg, err := range Scan(bytes.NewReader(frag)) {
		if err != nil {
			return nil, err
		}
		if seg.Marker == SOF0 {
			return ParseSOF(seg.Payload)
		}
		if seg.Marker == SOS {
			// SOS comes after SOF in well-formed JPEGs. Hitting SOS first
			// means no SOF — malformed.
			return nil, fmt.Errorf("%w: SOS before SOF", ErrBadJPEG)
		}
	}
	return nil, fmt.Errorf("%w: no SOF in fragment", ErrBadJPEG)
}

// findFirstSOFOffset walks frame by segments and returns the byte offset
// of the first FF C0 marker prefix (so the marker byte is at offset+1 and
// the length is at offset+2..+4).
func findFirstSOFOffset(frame []byte) (int, error) {
	// Reproduce the positional accounting from jpeg.Scan by tracking bytes
	// as we iterate. Scan yields parsed segments but hides positions; walk
	// the bytes directly instead.
	pos := 0
	for pos < len(frame)-1 {
		if frame[pos] != 0xFF {
			return -1, fmt.Errorf("%w: expected 0xFF at pos %d, got %02X", ErrBadJPEG, pos, frame[pos])
		}
		// Skip fill bytes.
		start := pos
		for pos < len(frame) && frame[pos] == 0xFF {
			pos++
		}
		if pos >= len(frame) {
			return -1, fmt.Errorf("%w: truncated at marker code", ErrBadJPEG)
		}
		code := Marker(frame[pos])
		if code == SOF0 {
			// marker prefix is at pos-1 (0xFF) — return the FF offset.
			return pos - 1, nil
		}
		pos++
		if code == 0x00 {
			return -1, fmt.Errorf("%w: 0xFF 00 outside scan data at pos %d", ErrBadJPEG, start)
		}
		if code.isStandalone() {
			continue
		}
		if pos+2 > len(frame) {
			return -1, fmt.Errorf("%w: truncated length at pos %d", ErrBadJPEG, pos)
		}
		segLen := int(binary.BigEndian.Uint16(frame[pos : pos+2]))
		if segLen < 2 {
			return -1, fmt.Errorf("%w: segment length %d < 2", ErrBadJPEG, segLen)
		}
		if code == SOS {
			// Past SOF; SOF missing in segment-walk region.
			return -1, fmt.Errorf("%w: SOS reached without SOF", ErrBadJPEG)
		}
		pos += segLen
		if pos > len(frame) {
			return -1, fmt.Errorf("%w: segment past frame end", ErrBadJPEG)
		}
	}
	return -1, fmt.Errorf("%w: no SOF in frame", ErrBadJPEG)
}

// initialCap pre-sizes the output buffer to avoid repeated grow-copy during
// append. Sum of fragment sizes is an upper bound; the real output is
// slightly larger (tables insert + optional APP14 + optional DRI) but the
// amortized cost of one extra grow is negligible next to the copy itself.
func initialCap(fragments [][]byte) int {
	n := 0
	for _, f := range fragments {
		n += len(f)
	}
	// small pad for tables splice; cheap.
	return n + 512
}
