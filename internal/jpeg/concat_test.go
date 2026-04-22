package jpeg

import (
	"bytes"
	"testing"
)

// fakeScan constructs a "fragment" that looks like a single-SOS JPEG scan:
// SOI + DQT + DHT + SOF + SOS + scan_data + EOI. ConcatenateScans will
// extract the entropy-coded part (the bytes between SOS's payload end and
// the next non-RST marker) from each fragment.
func fakeScan(t *testing.T, width, height uint16, scanData []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	// DQT: marker + len=3 + class/id=0 + 1 byte quant value
	buf.Write([]byte{0xFF, 0xDB, 0x00, 0x03, 0x00})
	// DHT: marker + len=3 + class/id=0x10 + 1 byte symbol length count
	buf.Write([]byte{0xFF, 0xC4, 0x00, 0x03, 0x10})
	// SOF
	sof := BuildSOF(&SOF{
		Precision: 8, Width: width, Height: height,
		Components: []SOFComponent{
			{ID: 1, SamplingH: 1, SamplingV: 1, QuantTableID: 0},
		},
	})
	buf.Write(sof)
	// SOS: marker + len=8 + 1 component + id=1 + 0x00 + Ss=0 + Se=63 + Ah/Al=0
	buf.Write([]byte{0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00})
	// Scan data (byte-stuffed)
	buf.Write(scanData)
	// EOI
	buf.Write([]byte{0xFF, 0xD9})
	return buf.Bytes()
}

func TestConcatenateScansTwoFragments(t *testing.T) {
	frag1 := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	frag2 := fakeScan(t, 16, 8, []byte{0x33, 0x44})

	jpegtables := []byte{
		0xFF, 0xD8,                   // SOI
		0xFF, 0xDB, 0x00, 0x03, 0x55, // DQT with different quant value
		0xFF, 0xC4, 0x00, 0x03, 0x20, // DHT
		0xFF, 0xD9,                   // EOI
	}
	out, err := ConcatenateScans(
		[][]byte{frag1, frag2},
		ConcatOpts{Width: 16, Height: 16, JPEGTables: jpegtables, RestartInterval: 1},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}
	// Verify the output is well-formed by walking segments.
	var markers []Marker
	for seg, err := range Scan(bytes.NewReader(out)) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		markers = append(markers, seg.Marker)
		if seg.Marker == SOS {
			break
		}
	}
	// Expected order: SOI, DQT (from tables), DHT (from tables), SOF, DRI, SOS
	want := []Marker{SOI, DQT, DHT, SOF0, DRI, SOS}
	if len(markers) != len(want) {
		t.Fatalf("segment order: got %v, want %v", markers, want)
	}
	for i := range markers {
		if markers[i] != want[i] {
			t.Errorf("segment %d: got 0x%X, want 0x%X", i, markers[i], want[i])
		}
	}
	// The tail should be ...scan1 + RST0 + scan2 + EOI.
	// Find the last marker (EOI) position.
	if out[len(out)-2] != 0xFF || Marker(out[len(out)-1]) != EOI {
		t.Errorf("final bytes: got 0x%X 0x%X, want FF D9", out[len(out)-2], out[len(out)-1])
	}
}

func TestConcatenateScansRejectsEmptyFragments(t *testing.T) {
	_, err := ConcatenateScans(nil, ConcatOpts{Width: 1, Height: 1})
	if err == nil {
		t.Fatal("expected error on empty fragments")
	}
}

func TestConcatenateScansColorspaceFix(t *testing.T) {
	// Verify the APP14 Adobe segment is emitted correctly when
	// ColorspaceFix is true.
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	jpegtables := []byte{
		0xFF, 0xD8,
		0xFF, 0xDB, 0x00, 0x03, 0x00,
		0xFF, 0xC4, 0x00, 0x03, 0x10,
		0xFF, 0xD9,
	}
	out, err := ConcatenateScans(
		[][]byte{frag},
		ConcatOpts{Width: 16, Height: 8, JPEGTables: jpegtables, ColorspaceFix: true},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}
	// The APP14 segment should appear immediately after SOI.
	// Bytes 0..1 = SOI, bytes 2..17 = APP14 (16 bytes).
	wantAPP14 := []byte{
		0xFF, 0xEE, 0x00, 0x0E,
		'A', 'd', 'o', 'b', 'e',
		0x64, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x00,
	}
	if len(out) < 18 {
		t.Fatalf("output too short: %d bytes", len(out))
	}
	if !bytes.Equal(out[2:18], wantAPP14) {
		t.Errorf("APP14 segment mismatch:\n got: %X\nwant: %X", out[2:18], wantAPP14)
	}
	// Walk segments; first two should be SOI then APP14.
	var markers []Marker
	for seg, err := range Scan(bytes.NewReader(out)) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		markers = append(markers, seg.Marker)
		if len(markers) == 2 {
			break
		}
	}
	if markers[0] != SOI || markers[1] != APP14 {
		t.Errorf("first two markers: got %X %X, want SOI APP14", markers[0], markers[1])
	}
}

// fakeScanWithAPP constructs a fragment that includes an APPn segment whose
// payload happens to contain 0xFF 0xDA bytes — the old byte-scan extractor
// would false-match on this. The proper segment walk skips the APP payload
// and locates the actual SOS correctly.
func fakeScanWithAPP(t *testing.T, width, height uint16, scanData []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// APP1 segment with a payload that contains FF DA (which would fool
	// a byte-level scanner into thinking it found SOS here).
	// APP1 marker = 0xE1. Length = 2 + 4 = 6. Payload: 0x00 0xFF 0xDA 0x00
	buf.Write([]byte{0xFF, 0xE1, 0x00, 0x06, 0x00, 0xFF, 0xDA, 0x00})
	// Real DQT + DHT + SOF + SOS + scan + EOI after the trap.
	buf.Write([]byte{0xFF, 0xDB, 0x00, 0x03, 0x00})
	buf.Write([]byte{0xFF, 0xC4, 0x00, 0x03, 0x10})
	sof := BuildSOF(&SOF{
		Precision: 8, Width: width, Height: height,
		Components: []SOFComponent{
			{ID: 1, SamplingH: 1, SamplingV: 1, QuantTableID: 0},
		},
	})
	buf.Write(sof)
	buf.Write([]byte{0xFF, 0xDA, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3F, 0x00})
	buf.Write(scanData)
	buf.Write([]byte{0xFF, 0xD9})
	return buf.Bytes()
}

func TestExtractScanDataSkipsAPPnPayload(t *testing.T) {
	frag := fakeScanWithAPP(t, 16, 8, []byte{0xAB, 0xCD})
	// Sanity: the fragment contains an FF DA inside APP1 payload AND an FF DA
	// SOS marker. Our extractor must locate the real SOS and return the
	// scan data, not the bytes that follow the APP1's decoy.
	jpegtables := []byte{
		0xFF, 0xD8,
		0xFF, 0xDB, 0x00, 0x03, 0x55,
		0xFF, 0xC4, 0x00, 0x03, 0x20,
		0xFF, 0xD9,
	}
	out, err := ConcatenateScans([][]byte{frag}, ConcatOpts{Width: 16, Height: 8, JPEGTables: jpegtables})
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}
	// The output must contain the real scan data (AB CD) and EOI.
	if !bytes.Contains(out, []byte{0xAB, 0xCD}) {
		t.Fatal("real scan data not found in output — false-matched APP1 decoy?")
	}
	if out[len(out)-2] != 0xFF || out[len(out)-1] != 0xD9 {
		t.Error("output does not end with EOI")
	}
}
