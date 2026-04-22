package jpeg

import (
	"bytes"
	"errors"
	"testing"
)

func TestScanSegmentsMinimal(t *testing.T) {
	// Minimal JPEG: SOI (FFD8), COM (FFFE) with 2-byte length+1-byte payload,
	// EOI (FFD9).
	data := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xFE, 0x00, 0x03, 'X', // COM length=3 (includes length bytes), payload 'X'
		0xFF, 0xD9, // EOI
	}
	var got []Marker
	for seg, err := range Scan(bytes.NewReader(data)) {
		if err != nil {
			t.Fatalf("scan err: %v", err)
		}
		got = append(got, seg.Marker)
	}
	want := []Marker{SOI, COM, EOI}
	if len(got) != len(want) {
		t.Fatalf("got markers %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("marker[%d]: got 0x%X, want 0x%X", i, got[i], want[i])
		}
	}
}

func TestScanStopsAtEOI(t *testing.T) {
	// Trailing bytes after EOI must not be consumed.
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9, 'J', 'U', 'N', 'K'}
	count := 0
	for _, err := range Scan(bytes.NewReader(data)) {
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		count++
	}
	if count != 2 {
		t.Fatalf("got %d markers, want 2 (SOI+EOI)", count)
	}
}

func TestScanSOSReturnsScanSegment(t *testing.T) {
	// After SOS, the iterator should yield a Segment with Marker=SOS and
	// Payload holding the SOS parameters (length minus 2 bytes). It does
	// NOT walk the entropy-coded scan data — caller uses ReadScan for that.
	data := []byte{
		0xFF, 0xD8,                   // SOI
		0xFF, 0xDA, 0x00, 0x08, 1, 2, 3, 4, 5, 6, // SOS len=8, payload 6 bytes
		// entropy-coded scan (byte-stuffed): the iterator should NOT read this
		0x11, 0x22, 0x33, 0xFF, 0x00, 0x44,
		0xFF, 0xD9,                   // EOI
	}
	var sosPayload []byte
	for seg, err := range Scan(bytes.NewReader(data)) {
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if seg.Marker == SOS {
			sosPayload = seg.Payload
			break // iterator lets us stop here; scan data reading is caller's job
		}
	}
	want := []byte{1, 2, 3, 4, 5, 6}
	if !bytes.Equal(sosPayload, want) {
		t.Fatalf("SOS payload: got %v, want %v", sosPayload, want)
	}
}

func TestScanRejectsBadMagic(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0xFF, 0xFF} // padding-like; no valid marker
	var gotErr error
	for _, err := range Scan(bytes.NewReader(data)) {
		if err != nil {
			gotErr = err
			break
		}
	}
	if !errors.Is(gotErr, ErrBadJPEG) {
		t.Fatalf("expected ErrBadJPEG, got %v", gotErr)
	}
}
