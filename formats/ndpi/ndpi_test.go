package ndpi

import (
	"bytes"
	"testing"

	"github.com/tcornish/opentile-go/internal/tiff"
)

// buildNDPIStub returns a tiny classic-TIFF with a SourceLens tag (65420).
// For this test we don't need the full NDPI 64-bit extension layout because
// we're only testing the Supports detection (which relies on the tiff.File
// already being parsed correctly by internal/tiff).
func buildNDPIStub(t *testing.T) []byte {
	t.Helper()
	// Build an NDPI-layout TIFF via the existing internal tiff test helper
	// pattern: classic magic 42, 12-byte first-IFD offset uint64 in header,
	// 5 entries including SourceLens (65420), next-IFD=0, 4-byte hi-bits
	// extension per tag.
	//
	// Layout:
	//   header (12 bytes):     II 42 00 <firstIFD uint64>
	//   at firstIFD = 12:      tagno u16 = 5
	//                          5 × 12-byte entries (60 bytes)
	//                          8-byte next-IFD = 0
	//                          5 × 4-byte hi-bits = 0
	buf := new(bytes.Buffer)
	w16 := func(v uint16) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
	}
	w32 := func(v uint32) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}
	w64 := func(v uint64) {
		w32(uint32(v))
		w32(uint32(v >> 32))
	}
	// Header (12 bytes — NDPI uses 8-byte first-IFD)
	buf.Write([]byte{'I', 'I', 42, 0})
	w64(12) // firstIFD = 12 (immediately after header)
	// IFD at offset 12
	w16(5)                                 // tagno = 5
	// Tag entries (12 bytes each)
	w16(256); w16(3); w32(1); w32(1024)    // ImageWidth (SHORT)
	w16(257); w16(3); w32(1); w32(768)     // ImageLength (SHORT)
	w16(322); w16(3); w32(1); w32(640)     // TileWidth (SHORT)
	w16(323); w16(3); w32(1); w32(8)       // TileLength (SHORT) — NDPI stripes are 8 tall
	w16(65420); w16(3); w32(1); w32(20)    // SourceLens (SHORT) = 20x
	// Next-IFD (8 bytes uint64) = 0
	w64(0)
	// Hi-bits extension (4 bytes × 5 tags) = all zeros for a small fixture
	for i := 0; i < 5; i++ {
		w32(0)
	}
	return buf.Bytes()
}

func TestSupportsDetectsNDPI(t *testing.T) {
	data := buildNDPIStub(t)
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if !New().Supports(f) {
		t.Fatal("Supports: expected true for SourceLens-bearing TIFF")
	}
}

func TestSupportsRejectsNonNDPI(t *testing.T) {
	// Same builder pattern but WITHOUT the SourceLens entry. This is a plain
	// classic TIFF (NOT NDPI mode). buildNonNDPIStub emits 4 tags and uses
	// the classic 4-byte first-IFD header.
	data := buildNonNDPIStub(t)
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if New().Supports(f) {
		t.Fatal("Supports: expected false for non-NDPI TIFF")
	}
}

func buildNonNDPIStub(t *testing.T) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	w16 := func(v uint16) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
	}
	w32 := func(v uint32) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0}) // classic 4-byte firstIFD = 8
	w16(4) // 4 tags
	w16(256); w16(3); w32(1); w32(1024)
	w16(257); w16(3); w32(1); w32(768)
	w16(322); w16(3); w32(1); w32(640)
	w16(323); w16(3); w32(1); w32(8)
	w32(0) // next-IFD = 0
	return buf.Bytes()
}
