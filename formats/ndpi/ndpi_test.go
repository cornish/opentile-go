package ndpi

import (
	"bytes"
	"testing"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// buildNDPIStub returns a tiny classic-TIFF with a SourceLens tag (65420).
// For this test we don't need the full NDPI 64-bit extension layout because
// we're only testing the Supports detection (which relies on the tiff.File
// already being parsed correctly by internal/tiff).
func buildNDPIStub(t *testing.T) []byte {
	t.Helper()
	// Build an NDPI-layout TIFF with 7 tags including TileOffsets/TileByteCounts
	// so that Open's newStripedImage can parse the page fully.
	//
	// Layout (all little-endian):
	//   header (12 bytes):       II 42 00 <firstIFD uint64>
	//   IFD at offset 12:        tagno u16 = 7
	//                            7 × 12-byte entries (84 bytes)
	//                            8-byte next-IFD = 0
	//                            7 × 4-byte hi-bits = 0
	//   IFD end = 12+2+84+8+28 = 134
	//   TileOffsets data:        96 × uint32 at offset 134 (384 bytes)
	//   TileByteCounts data:     96 × uint32 at offset 518 (384 bytes)
	//
	// Image: 640×768, TileWidth=640, TileLength=8 → grid 1×96 = 96 native stripes.
	// Tile offsets and counts are zero (tiles are never read in the Open test).
	const (
		nTags   = 7
		nTiles  = 96 // ceil(768/8) = 96 stripes
		ifdBase = 12
		// ifdEnd = ifdBase + 2 + nTags*12 + 8 + nTags*4 = 12+2+84+8+28 = 134
		offsetsDataOff = 134
		countsDataOff  = offsetsDataOff + nTiles*4 // 134 + 384 = 518
	)
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
	w64(ifdBase) // firstIFD = 12
	// IFD at offset 12
	w16(nTags) // tagno = 7
	// Tag entries (12 bytes each): tag(u16) type(u16) count(u32) value/offset(u32)
	w16(256); w16(3); w32(1); w32(1024)                // ImageWidth SHORT
	w16(257); w16(3); w32(1); w32(768)                 // ImageLength SHORT
	w16(322); w16(3); w32(1); w32(640)                 // TileWidth SHORT
	w16(323); w16(3); w32(1); w32(8)                   // TileLength SHORT (8-pixel stripes)
	w16(324); w16(4); w32(nTiles); w32(offsetsDataOff) // TileOffsets LONG[96]
	w16(325); w16(4); w32(nTiles); w32(countsDataOff)  // TileByteCounts LONG[96]
	w16(65420); w16(3); w32(1); w32(20)                // SourceLens SHORT = 20x
	// Next-IFD (8 bytes uint64) = 0
	w64(0)
	// Hi-bits extension (4 bytes × nTags) = all zeros for a small fixture
	for i := 0; i < nTags; i++ {
		w32(0)
	}
	// TileOffsets data: 96 × uint32 = 0 (tiles are never read in Open test)
	for i := 0; i < nTiles; i++ {
		w32(0)
	}
	// TileByteCounts data: 96 × uint32 = 0
	for i := 0; i < nTiles; i++ {
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

func TestNdpiOpenClassifiesPages(t *testing.T) {
	data := buildNDPIStub(t)
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	cfg := opentile.NewTestConfig(opentile.Size{W: 640, H: 640}, opentile.CorruptTileError)
	tiler, err := New().Open(f, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tiler.Close()
	if tiler.Format() != opentile.FormatNDPI {
		t.Errorf("Format: got %q, want %q", tiler.Format(), opentile.FormatNDPI)
	}
	if got := len(tiler.Levels()); got != 1 {
		t.Errorf("levels: got %d, want 1", got)
	}
	if got := len(tiler.Associated()); got != 0 {
		t.Errorf("associated: got %d, want 0", got)
	}
	md, ok := MetadataOf(tiler)
	if !ok {
		t.Fatal("MetadataOf returned ok=false")
	}
	if md.SourceLens != 20 {
		t.Errorf("SourceLens: got %v, want 20", md.SourceLens)
	}
}
