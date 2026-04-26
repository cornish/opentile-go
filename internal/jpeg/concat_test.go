package jpeg

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// fakeScan constructs a "fragment" that looks like an SVS-style abbreviated
// JPEG strip: SOI + SOF + SOS + scan_data + EOI. Python opentile appends the
// first such fragment whole and only the post-SOS bytes of subsequent
// fragments (see concatenate_scans port in concat.go). No DQT/DHT in the
// fragment itself; those come from the JPEGTables blob the caller passes to
// ConcatenateScans.
func fakeScan(t *testing.T, width, height uint16, scanData []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
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

// minimalJPEGTables returns a mini-JPEG "SOI DQT DHT EOI" whose tables[2:-2]
// is the inner DQT+DHT — exactly what Python's _add_jpeg_tables splices.
func minimalJPEGTables() []byte {
	return []byte{
		0xFF, 0xD8, // SOI
		// DQT: marker + len=3 + class/id=0 + 1 byte quant value
		0xFF, 0xDB, 0x00, 0x03, 0x55,
		// DHT: marker + len=3 + class/id=0x10 + 1 byte symbol count
		0xFF, 0xC4, 0x00, 0x03, 0x20,
		0xFF, 0xD9, // EOI
	}
}

// segmentMarkers walks the frame and returns the marker sequence up to and
// including the first SOS. Used to assert the Python-canonical ordering.
func segmentMarkers(t *testing.T, frame []byte) []Marker {
	t.Helper()
	var markers []Marker
	for seg, err := range Scan(bytes.NewReader(frame)) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		markers = append(markers, seg.Marker)
		if seg.Marker == SOS {
			return markers
		}
	}
	return markers
}

func TestConcatenateScansTwoFragments(t *testing.T) {
	// Two fragments, identical width, same internal SOF height. Accumulated
	// image size should be (W, 2*H). Python's layout before the first SOS:
	// SOI, SOF (first fragment's, patched to accumulated size), DQT, DHT
	// (from JPEGTables), DRI (since RestartInterval > 0), SOS.
	frag1 := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	frag2 := fakeScan(t, 16, 8, []byte{0x33, 0x44})
	tables := minimalJPEGTables()

	out, err := ConcatenateScans(
		[][]byte{frag1, frag2},
		ConcatOpts{JPEGTables: tables, RestartInterval: 1},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}

	// Segment order before first SOS should be: SOI, SOF0, DQT, DHT, DRI, SOS.
	want := []Marker{SOI, SOF0, DQT, DHT, DRI, SOS}
	got := segmentMarkers(t, out)
	if len(got) != len(want) {
		t.Fatalf("segment order: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("segment %d: got 0x%X, want 0x%X", i, got[i], want[i])
		}
	}

	// SOF should advertise the accumulated height (16) and first-fragment
	// width (16).
	for seg, err := range Scan(bytes.NewReader(out)) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if seg.Marker == SOF0 {
			s, err := ParseSOF(seg.Payload)
			if err != nil {
				t.Fatalf("parse SOF: %v", err)
			}
			if s.Width != 16 || s.Height != 16 {
				t.Errorf("SOF size: got %dx%d, want 16x16", s.Width, s.Height)
			}
			break
		}
	}

	// The frame should end with ...scan1 [FF RST0] scan2 [FF D9].
	if out[len(out)-2] != 0xFF || out[len(out)-1] != 0xD9 {
		t.Errorf("trailing bytes: got %02X %02X, want FF D9", out[len(out)-2], out[len(out)-1])
	}
	// The FF D9 boundary between the two fragments' data should have been
	// rewritten to FF D0 (RST0). Locate by scanning for both scan bodies.
	idx1 := bytes.Index(out, []byte{0x11, 0x22})
	idx2 := bytes.Index(out, []byte{0x33, 0x44})
	if idx1 < 0 || idx2 < 0 || idx2 < idx1 {
		t.Fatalf("scan payloads not found in order: %d %d", idx1, idx2)
	}
	// Between 0x11 0x22 and 0x33 0x44 we should see FF D0 (RST0) — at the
	// position of the original FF D9 of the first fragment.
	between := out[idx1+2 : idx2]
	if !bytes.Contains(between, []byte{0xFF, 0xD0}) {
		t.Errorf("expected FF D0 (RST0) between fragments, found %X", between)
	}
}

func TestConcatenateScansRejectsEmptyFragments(t *testing.T) {
	_, err := ConcatenateScans(nil, ConcatOpts{})
	if err == nil {
		t.Fatal("expected error on empty fragments")
	}
}

func TestConcatenateScansColorspaceFix(t *testing.T) {
	// ColorspaceFix emits the Adobe APP14 segment AFTER the inserted
	// tables but BEFORE the DRI/SOS — Python's _add_jpeg_tables_and_rgb_
	// color_space_fix appends APP14 to the tables insert, then
	// _manipulate_header inserts DRI before SOS.
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	tables := minimalJPEGTables()

	out, err := ConcatenateScans(
		[][]byte{frag},
		ConcatOpts{JPEGTables: tables, ColorspaceFix: true, RestartInterval: 1},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}

	// Segment order: SOI, SOF0, DQT, DHT, APP14, DRI, SOS.
	want := []Marker{SOI, SOF0, DQT, DHT, APP14, DRI, SOS}
	got := segmentMarkers(t, out)
	if len(got) != len(want) {
		t.Fatalf("segment order: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("segment %d: got 0x%X, want 0x%X", i, got[i], want[i])
		}
	}

	// APP14 bytes must match the canonical Adobe literal. Locate the APP14
	// segment by scanning for FF EE after SOI.
	app14Pos := bytes.Index(out, []byte{0xFF, 0xEE})
	if app14Pos < 0 {
		t.Fatal("APP14 segment not found in output")
	}
	if !bytes.Equal(out[app14Pos:app14Pos+len(adobeAPP14)], adobeAPP14) {
		t.Errorf("APP14 bytes mismatch:\n got %X\nwant %X",
			out[app14Pos:app14Pos+len(adobeAPP14)], adobeAPP14)
	}
}

func TestConcatenateScansNoRestartInterval(t *testing.T) {
	// RestartInterval=0: no DRI in the output. Fragment boundary still gets
	// a FF RSTn marker, matching Python (_manipulate_header is called with
	// restart_interval=None by callers that don't pass one, but concatenate_
	// scans does pass one; this test covers callers — e.g. potential future
	// code — that request no DRI explicitly).
	frag1 := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	frag2 := fakeScan(t, 16, 8, []byte{0x33, 0x44})
	tables := minimalJPEGTables()

	out, err := ConcatenateScans(
		[][]byte{frag1, frag2},
		ConcatOpts{JPEGTables: tables, RestartInterval: 0},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}

	// No DRI.
	if bytes.Contains(out, []byte{0xFF, 0xDD}) {
		t.Error("unexpected DRI in output when RestartInterval=0")
	}
}

func TestConcatenateScansExplicitSize(t *testing.T) {
	// opts.Width/Height override the accumulated defaults. This matches
	// the SVS associated-image caller, which knows the final image size
	// from the TIFF ImageWidth/ImageLength tags and doesn't need to rely
	// on fragment-header summation.
	frag := fakeScan(t, 100, 50, []byte{0x11, 0x22})
	tables := minimalJPEGTables()

	out, err := ConcatenateScans(
		[][]byte{frag},
		ConcatOpts{Width: 1234, Height: 5678, JPEGTables: tables},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}
	// Parse SOF payload; verify height=5678, width=1234.
	for seg, err := range Scan(bytes.NewReader(out)) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if seg.Marker == SOF0 {
			s, err := ParseSOF(seg.Payload)
			if err != nil {
				t.Fatalf("parse SOF: %v", err)
			}
			if s.Width != 1234 || s.Height != 5678 {
				t.Errorf("SOF size: got %dx%d, want 1234x5678", s.Width, s.Height)
			}
			return
		}
	}
	t.Fatal("SOF not found in output")
}

// TestConcatenateScansSingleFragmentWithTrailingZeros verifies that a single
// fragment whose last bytes are 00 00 (zero padding, as seen in Aperio BigTIFF
// SVS thumbnails whose StripByteCounts is over-allocated) is accepted and the
// output ends with FF D9. Upstream Python opentile blindly overwrites the last
// 2 bytes with EOI, so trailing padding is harmless.
func TestConcatenateScansSingleFragmentWithTrailingZeros(t *testing.T) {
	// Build a fragment that ends with 100 zero bytes instead of FF D9.
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	// Replace the trailing FF D9 with 00 00, then pad with 98 more zeros.
	// fakeScan ends with [...scan_data, FF, D9]. Strip the EOI and append zeros.
	frag = frag[:len(frag)-2]
	padding := make([]byte, 100)
	frag = append(frag, padding...)
	// frag now ends with 100 zero bytes; no EOI at the end.

	out, err := ConcatenateScans([][]byte{frag}, ConcatOpts{JPEGTables: minimalJPEGTables()})
	if err != nil {
		t.Fatalf("ConcatenateScans with trailing zeros: %v", err)
	}
	// Output length will be >= input length due to tables splice; we only
	// care that the output ends with FF D9 (EOI was written unconditionally).
	if out[len(out)-2] != 0xFF || out[len(out)-1] != 0xD9 {
		t.Errorf("last 2 bytes: got %02X %02X, want FF D9", out[len(out)-2], out[len(out)-1])
	}
}

// TestConcatenateScansSingleFragmentEndsWithEOI verifies the existing happy path
// still works: a single fragment with a proper FF D9 tail is returned unchanged
// (modulo tables splice/SOF patch).
func TestConcatenateScansSingleFragmentEndsWithEOI(t *testing.T) {
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	// frag ends with FF D9.
	tables := minimalJPEGTables()
	out, err := ConcatenateScans([][]byte{frag}, ConcatOpts{JPEGTables: tables})
	if err != nil {
		t.Fatalf("ConcatenateScans with proper EOI: %v", err)
	}
	if out[len(out)-2] != 0xFF || out[len(out)-1] != 0xD9 {
		t.Errorf("last 2 bytes: got %02X %02X, want FF D9", out[len(out)-2], out[len(out)-1])
	}
}

// TestConcatenateScansDRIValue sanity-checks the DRI payload uses the
// specified restart interval as a big-endian u16.
func TestConcatenateScansDRIValue(t *testing.T) {
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})
	tables := minimalJPEGTables()
	out, err := ConcatenateScans(
		[][]byte{frag},
		ConcatOpts{JPEGTables: tables, RestartInterval: 1234},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}
	driPos := bytes.Index(out, []byte{0xFF, 0xDD})
	if driPos < 0 {
		t.Fatal("DRI not found")
	}
	// FF DD 00 04 <payload>.
	payload := binary.BigEndian.Uint16(out[driPos+4 : driPos+6])
	if payload != 1234 {
		t.Errorf("DRI payload: got %d, want 1234", payload)
	}
}

// TestConcatenateScansEmptyJPEGTablesWithColorspaceFix locks in the L18 fix:
// when JPEGTables is absent (nil/empty), ConcatenateScans must skip BOTH the
// tables splice AND the APP14 marker, even if ColorspaceFix=true. This matches
// upstream Python opentile (opentile/jpeg/jpeg.py:192-198):
//
//	if jpeg_tables is not None:
//	    if rgb_colorspace_fix:
//	        frame = self._add_jpeg_tables_and_rgb_color_space_fix(frame, jpeg_tables)
//	    else:
//	        frame = self._add_jpeg_tables(frame, jpeg_tables)
//
// The rgb_colorspace_fix branch is gated INSIDE the tables-present check; when
// tables are absent the colorspace fix is silently skipped too. Required for
// SVS files emitted by scanners (e.g. Grundium) that omit the shared
// JPEGTables tag — strips embed their own DQT/DHT/SOF inline.
func TestConcatenateScansEmptyJPEGTablesWithColorspaceFix(t *testing.T) {
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})

	out, err := ConcatenateScans(
		[][]byte{frag},
		ConcatOpts{JPEGTables: nil, ColorspaceFix: true, RestartInterval: 1},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}

	// Segment order must be SOI, SOF0, DRI, SOS — no DQT/DHT (no tables) and
	// no APP14 (no colorspace splice when tables are absent).
	want := []Marker{SOI, SOF0, DRI, SOS}
	got := segmentMarkers(t, out)
	if len(got) != len(want) {
		t.Fatalf("segment order: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("segment %d: got 0x%X, want 0x%X", i, got[i], want[i])
		}
	}

	// Belt-and-braces: APP14 (FF EE) must not appear anywhere in the output.
	if bytes.Contains(out, []byte{0xFF, 0xEE}) {
		t.Error("APP14 (FF EE) present in output despite empty JPEGTables; upstream gates APP14 inside the tables-present branch")
	}
}

// TestConcatenateScansEmptyJPEGTablesNoColorspaceFix is a regression guard:
// the absence of any tables-related splice was already correct before the L18
// fix (the bug was specifically in the ColorspaceFix=true branch). This locks
// in that the no-tables, no-colorspace-fix path stays a no-op splice.
func TestConcatenateScansEmptyJPEGTablesNoColorspaceFix(t *testing.T) {
	frag := fakeScan(t, 16, 8, []byte{0x11, 0x22})

	out, err := ConcatenateScans(
		[][]byte{frag},
		ConcatOpts{JPEGTables: nil, ColorspaceFix: false, RestartInterval: 1},
	)
	if err != nil {
		t.Fatalf("ConcatenateScans: %v", err)
	}
	want := []Marker{SOI, SOF0, DRI, SOS}
	got := segmentMarkers(t, out)
	if len(got) != len(want) {
		t.Fatalf("segment order: got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("segment %d: got 0x%X, want 0x%X", i, got[i], want[i])
		}
	}
}
