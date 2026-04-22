package jpeg

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ConcatOpts controls how ConcatenateScans assembles the output JPEG.
type ConcatOpts struct {
	Width, Height   uint16 // output SOF dimensions
	JPEGTables      []byte // raw TIFF JPEGTables value; DQT/DHT extracted via SplitJPEGTables
	ColorspaceFix   bool   // if true, emit an APP14 "Adobe" segment signalling RGB (for SVS non-standard RGB JPEGs)
	RestartInterval int    // 0 = no DRI; otherwise emit DRI and insert RST markers between fragment scans
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

// extractScanData walks frag, finds the SOS marker by byte scan, and returns
// the entropy-coded bytes up to (but not including) the trailing EOI. Byte
// stuffing is preserved. This byte-level approach avoids chained-reader issues
// that arise when using Scan (which wraps its io.Reader in a new bufio.Reader
// internally) followed by ReadScan.
func extractScanData(frag []byte) ([]byte, error) {
	// Find SOS marker by byte scan.
	pos := -1
	for i := 0; i < len(frag)-1; i++ {
		if frag[i] == 0xFF && Marker(frag[i+1]) == SOS {
			pos = i
			break
		}
	}
	if pos < 0 {
		return nil, fmt.Errorf("%w: no SOS found", ErrBadJPEG)
	}
	// SOS header: 2 marker bytes + 2-byte length + payload.
	if pos+4 > len(frag) {
		return nil, fmt.Errorf("%w: SOS truncated", ErrBadJPEG)
	}
	sosLen := int(binary.BigEndian.Uint16(frag[pos+2 : pos+4]))
	scanStart := pos + 2 + sosLen
	if scanStart > len(frag) {
		return nil, fmt.Errorf("%w: scan start past frag end", ErrBadJPEG)
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
