package jpeg

import (
	"bytes"
	"testing"
)

func TestReadScanStripsNothing(t *testing.T) {
	// Entropy data with a stuffed byte (FF 00) and an RST1 in the middle.
	// ReadScan should return the raw bytes including stuffed 00, stopping
	// just before the next non-RST marker.
	scan := []byte{0x11, 0x22, 0xFF, 0x00, 0x33, 0xFF, 0xD1 /*RST1*/, 0x44, 0xFF, 0xD9 /*EOI*/}
	r := bytes.NewReader(scan)
	got, next, err := ReadScan(r)
	if err != nil {
		t.Fatalf("ReadScan: %v", err)
	}
	want := []byte{0x11, 0x22, 0xFF, 0x00, 0x33, 0xFF, 0xD1, 0x44}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if next != EOI {
		t.Errorf("next marker: got 0x%X, want 0x%X", next, EOI)
	}
}

func TestReadScanStopsAtSOF(t *testing.T) {
	scan := []byte{0x11, 0xFF, 0xC0, 0x00, 0x08, 1, 2, 3, 4, 5, 6}
	r := bytes.NewReader(scan)
	got, next, err := ReadScan(r)
	if err != nil {
		t.Fatalf("ReadScan: %v", err)
	}
	if !bytes.Equal(got, []byte{0x11}) {
		t.Fatalf("scan data: got %v, want [0x11]", got)
	}
	if next != SOF0 {
		t.Errorf("next marker: got 0x%X, want 0x%X", next, SOF0)
	}
}
