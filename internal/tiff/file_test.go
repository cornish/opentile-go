package tiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestOpenFileMinimal(t *testing.T) {
	data := buildClassicTIFF(t, [][3]uint32{
		{256, 3, 1024}, // ImageWidth
		{257, 3, 768},  // ImageLength
	})
	f, err := Open(bytes.NewReader(data), int64(len(data)))
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
	if _, err := Open(bytes.NewReader(data), int64(len(data))); err == nil {
		t.Fatal("expected BigTIFF to be rejected")
	}
}

// TestPageAtOffset confirms that a single IFD can be read at an arbitrary
// offset via PageAtOffset, used by OME TIFF for SubIFD traversal (SubIFDs
// are reachable only via tag 330, not the top-level next-IFD chain).
func TestPageAtOffset(t *testing.T) {
	// Build a TIFF whose top-level chain is one IFD (ImageWidth=1024,
	// ImageLength=768), followed by a second IFD body laid at a known
	// offset that's NOT reachable via the next-IFD chain. PageAtOffset
	// on that offset should return the second IFD as a Page.
	first := buildClassicTIFF(t, [][3]uint32{
		{256, 3, 1024},
		{257, 3, 768},
	})
	subOffset := uint32(len(first))
	// Append a second IFD at subOffset: 1 entry (ImageWidth=512), next=0.
	second := new(bytes.Buffer)
	_ = binary.Write(second, binary.LittleEndian, uint16(1)) // entry count
	_ = binary.Write(second, binary.LittleEndian, uint16(256))
	_ = binary.Write(second, binary.LittleEndian, uint16(3))
	_ = binary.Write(second, binary.LittleEndian, uint32(1))
	_ = binary.Write(second, binary.LittleEndian, uint32(512))
	_ = binary.Write(second, binary.LittleEndian, uint32(0)) // next IFD
	data := append(first, second.Bytes()...)

	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(f.Pages()) != 1 {
		t.Fatalf("top-level pages: got %d, want 1 (sub-IFD must NOT be in the top-level chain)", len(f.Pages()))
	}
	sub, err := f.PageAtOffset(uint64(subOffset))
	if err != nil {
		t.Fatalf("PageAtOffset: %v", err)
	}
	iw, ok := sub.ImageWidth()
	if !ok || iw != 512 {
		t.Errorf("sub-page ImageWidth: got (%d, %v), want (512, true)", iw, ok)
	}
}
