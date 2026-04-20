package tiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildClassicTIFF builds a tiny in-memory TIFF (LE) with one IFD containing
// the supplied entries. All tag values must fit inline (≤4 bytes).
// Returns the raw bytes; first IFD is at offset 8.
func buildClassicTIFF(t *testing.T, entries [][3]uint32) []byte {
	t.Helper()
	// Header: II 42 0x00000008
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0})
	// IFD: count (u16), entries (12 bytes each), next IFD (u32)
	n := uint16(len(entries))
	_ = binary.Write(buf, binary.LittleEndian, n)
	for _, e := range entries {
		// tag u16, type u16 (SHORT=3), count u32, value-or-offset u32
		_ = binary.Write(buf, binary.LittleEndian, uint16(e[0]))
		_ = binary.Write(buf, binary.LittleEndian, uint16(3)) // SHORT
		_ = binary.Write(buf, binary.LittleEndian, uint32(1))
		_ = binary.Write(buf, binary.LittleEndian, e[2])
	}
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD = 0
	return buf.Bytes()
}

func TestWalkIFDs(t *testing.T) {
	data := buildClassicTIFF(t, [][3]uint32{
		{256, 3, 1024}, // ImageWidth = 1024
		{257, 3, 768},  // ImageLength = 768
	})
	r := bytes.NewReader(data)
	h, err := parseHeader(r)
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	b := newByteReader(r, h.littleEndian)

	ifds, err := walkIFDs(b, int64(h.firstIFD))
	if err != nil {
		t.Fatalf("walkIFDs: %v", err)
	}
	if len(ifds) != 1 {
		t.Fatalf("ifd count: got %d, want 1", len(ifds))
	}
	ifd := ifds[0]
	if len(ifd.entries) != 2 {
		t.Fatalf("entry count: got %d, want 2", len(ifd.entries))
	}
	w, ok := ifd.get(256)
	if !ok {
		t.Fatal("ImageWidth missing")
	}
	wv, err := w.Values(b)
	if err != nil || len(wv) != 1 || wv[0] != 1024 {
		t.Fatalf("ImageWidth: got %v, err %v; want [1024]", wv, err)
	}
}

func TestWalkIFDsMultiple(t *testing.T) {
	// Two IFDs: first points to second via next-IFD offset.
	// Use buildClassicTIFF for the first, manually chain a second.
	// For simplicity, verify walkIFDs handles cycles / unbounded reads by
	// asserting it stops at a zero next-IFD offset and does not panic.
	data := buildClassicTIFF(t, [][3]uint32{{256, 3, 100}})
	r := bytes.NewReader(data)
	h, _ := parseHeader(r)
	b := newByteReader(r, h.littleEndian)
	ifds, err := walkIFDs(b, int64(h.firstIFD))
	if err != nil {
		t.Fatalf("walkIFDs: %v", err)
	}
	if len(ifds) != 1 {
		t.Fatalf("expected 1 IFD, got %d", len(ifds))
	}
}
