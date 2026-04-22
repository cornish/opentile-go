package jpeg

import (
	"bytes"
	"testing"
)

func TestSplitJPEGTables(t *testing.T) {
	// Synthetic JPEGTables blob: SOI + DQT + DHT + DHT + EOI.
	// DQT: FFDB 0003 00 → one table class/id=0, payload is 1 byte
	// DHT: FFC4 0003 10 → one Huffman DC table id=0, payload 1 byte (malformed
	//      but fine for splitting — we're not decoding)
	tables := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xDB, 0x00, 0x03, 0x00,          // DQT class=0 id=0 (1 byte payload)
		0xFF, 0xC4, 0x00, 0x03, 0x10,          // DHT class=1 id=0
		0xFF, 0xC4, 0x00, 0x03, 0x00,          // DHT class=0 id=0
		0xFF, 0xD9,                            // EOI
	}
	dqts, dhts, err := SplitJPEGTables(tables)
	if err != nil {
		t.Fatalf("SplitJPEGTables: %v", err)
	}
	if len(dqts) != 1 {
		t.Fatalf("dqts: got %d, want 1", len(dqts))
	}
	if len(dhts) != 2 {
		t.Fatalf("dhts: got %d, want 2", len(dhts))
	}
	// Each returned segment is the full bytes INCLUDING the marker and
	// length, suitable for concatenation into a new bitstream.
	wantDQT := []byte{0xFF, 0xDB, 0x00, 0x03, 0x00}
	if !bytes.Equal(dqts[0], wantDQT) {
		t.Errorf("dqt[0]: got %v, want %v", dqts[0], wantDQT)
	}
}

func TestSplitJPEGTablesRejectsNoSOI(t *testing.T) {
	_, _, err := SplitJPEGTables([]byte{0xFF, 0xDB, 0, 3, 0})
	if err == nil {
		t.Fatal("expected error on missing SOI")
	}
}

func TestSplitJPEGTablesIgnoresUnknownSegments(t *testing.T) {
	// A COM segment in the middle should be tolerated but not returned.
	tables := []byte{
		0xFF, 0xD8,
		0xFF, 0xFE, 0x00, 0x04, 'x', 'y',      // COM
		0xFF, 0xDB, 0x00, 0x03, 0x00,          // DQT
		0xFF, 0xD9,
	}
	dqts, dhts, err := SplitJPEGTables(tables)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(dqts) != 1 || len(dhts) != 0 {
		t.Errorf("got dqts=%d dhts=%d, want 1/0", len(dqts), len(dhts))
	}
}
