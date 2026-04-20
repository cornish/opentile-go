package tiff

import (
	"bytes"
	"testing"
)

func TestOpenFileMinimal(t *testing.T) {
	data := buildClassicTIFF(t, [][3]uint32{
		{256, 3, 1024}, // ImageWidth
		{257, 3, 768},  // ImageLength
	})
	f, err := Open(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(f.Pages()) != 1 {
		t.Fatalf("pages: got %d, want 1", len(f.Pages()))
	}
	if !f.LittleEndian() {
		t.Error("expected LittleEndian true")
	}
}

func TestOpenRejectsBigTIFF(t *testing.T) {
	data := []byte{'I', 'I', 43, 0, 8, 0, 0, 0, 0, 0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0}
	if _, err := Open(bytes.NewReader(data)); err == nil {
		t.Fatal("expected BigTIFF to be rejected")
	}
}
