package tiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildBigTIFF constructs a tiny BigTIFF with a single IFD containing one
// SHORT entry (tag=256, value=1024). All tag values fit inline in the 8-byte
// cell.
func buildBigTIFF(t *testing.T, entries [][3]uint64) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	// BigTIFF header: II 43 offsetSize=8 constant=0 firstIFD=0x10
	buf.Write([]byte{'I', 'I', 0x2B, 0x00, 0x08, 0x00, 0x00, 0x00})
	_ = binary.Write(buf, binary.LittleEndian, uint64(0x10))
	// IFD at offset 0x10: count(u64), entries (20 bytes each), next IFD (u64)
	_ = binary.Write(buf, binary.LittleEndian, uint64(len(entries)))
	for _, e := range entries {
		_ = binary.Write(buf, binary.LittleEndian, uint16(e[0]))
		_ = binary.Write(buf, binary.LittleEndian, uint16(3)) // SHORT
		_ = binary.Write(buf, binary.LittleEndian, uint64(1))
		_ = binary.Write(buf, binary.LittleEndian, e[2]) // value-or-offset (8 bytes)
	}
	_ = binary.Write(buf, binary.LittleEndian, uint64(0)) // next IFD = 0
	return buf.Bytes()
}

func TestWalkBigIFDs(t *testing.T) {
	data := buildBigTIFF(t, [][3]uint64{
		{256, 3, 1024}, // ImageWidth = 1024
		{257, 3, 768},  // ImageLength = 768
	})
	r := bytes.NewReader(data)
	h, err := parseHeader(r)
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if !h.bigTIFF {
		t.Fatal("expected bigTIFF=true")
	}
	b := newByteReader(r, h.littleEndian)
	ifds, err := walkIFDs(b, int64(h.firstIFD), h.bigTIFF)
	if err != nil {
		t.Fatalf("walkIFDs: %v", err)
	}
	if len(ifds) != 1 {
		t.Fatalf("ifd count: got %d, want 1", len(ifds))
	}
	e, ok := ifds[0].get(256)
	if !ok {
		t.Fatal("ImageWidth missing")
	}
	if e.Count != 1 {
		t.Errorf("count: got %d, want 1", e.Count)
	}
	if e.inlineCap != 8 {
		t.Errorf("inlineCap: got %d, want 8", e.inlineCap)
	}
	vals, err := e.Values(b)
	if err != nil || len(vals) != 1 || vals[0] != 1024 {
		t.Fatalf("ImageWidth: got %v, err %v", vals, err)
	}
}
