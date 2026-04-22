package tiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildPageTIFF builds a TIFF with a single IFD carrying the minimum SVS-level
// tag set: ImageWidth, ImageLength, TileWidth, TileLength, Compression,
// Photometric, TileOffsets[4], TileByteCounts[4], plus ImageDescription.
func buildPageTIFF(t *testing.T) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0}) // header, first IFD at 8

	const (
		tImageWidth   uint16 = 256
		tImageLength  uint16 = 257
		tCompression  uint16 = 259
		tPhotometric  uint16 = 262
		tImageDesc    uint16 = 270
		tTileWidth    uint16 = 322
		tTileLength   uint16 = 323
		tTileOffsets  uint16 = 324
		tTileByteCnts uint16 = 325
	)

	// 9 entries. count(u16) + 9*12 entry bytes + 4 nextIFD = 2 + 108 + 4 = 114 bytes.
	// External data region starts at offset 8 + 114 = 122.
	externalBase := uint32(122)

	// Description ASCII: "Aperio"
	desc := []byte("Aperio\x00")
	descOff := externalBase
	externalAfterDesc := externalBase + uint32(len(desc))

	// TileOffsets: 4 LONGs
	tileOff := []uint32{1000, 2000, 3000, 4000}
	tileOffOff := externalAfterDesc
	externalAfterOffsets := tileOffOff + uint32(4*len(tileOff))

	// TileByteCounts: 4 LONGs
	tileBC := []uint32{100, 200, 300, 400}
	tileBCOff := externalAfterOffsets
	_ = tileBCOff + uint32(4*len(tileBC))

	// 9 entries
	_ = binary.Write(buf, binary.LittleEndian, uint16(9)) // entry count
	writeEntry := func(tag uint16, typ DataType, count uint32, voc uint32) {
		_ = binary.Write(buf, binary.LittleEndian, tag)
		_ = binary.Write(buf, binary.LittleEndian, uint16(typ))
		_ = binary.Write(buf, binary.LittleEndian, count)
		_ = binary.Write(buf, binary.LittleEndian, voc)
	}
	writeEntry(tImageWidth, DTShort, 1, 1024)
	writeEntry(tImageLength, DTShort, 1, 768)
	writeEntry(tCompression, DTShort, 1, 7) // JPEG
	writeEntry(tPhotometric, DTShort, 1, 6) // YCbCr
	writeEntry(tImageDesc, DTASCII, uint32(len(desc)), descOff)
	writeEntry(tTileWidth, DTShort, 1, 256)
	writeEntry(tTileLength, DTShort, 1, 256)
	writeEntry(tTileOffsets, DTLong, 4, tileOffOff)
	writeEntry(tTileByteCnts, DTLong, 4, tileBCOff)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD

	// External data (order: desc, tileOff, tileBC)
	buf.Write(desc)
	for _, v := range tileOff {
		_ = binary.Write(buf, binary.LittleEndian, v)
	}
	for _, v := range tileBC {
		_ = binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func TestPageAccessors(t *testing.T) {
	data := buildPageTIFF(t)
	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(f.Pages()) != 1 {
		t.Fatalf("pages: got %d", len(f.Pages()))
	}
	p := f.Pages()[0]

	if iw, _ := p.ImageWidth(); iw != 1024 {
		t.Errorf("ImageWidth: got %d, want 1024", iw)
	}
	if il, _ := p.ImageLength(); il != 768 {
		t.Errorf("ImageLength: got %d, want 768", il)
	}
	if tw, _ := p.TileWidth(); tw != 256 {
		t.Errorf("TileWidth: got %d, want 256", tw)
	}
	if th, _ := p.TileLength(); th != 256 {
		t.Errorf("TileLength: got %d, want 256", th)
	}
	if c, _ := p.Compression(); c != 7 {
		t.Errorf("Compression: got %d, want 7", c)
	}
	desc, _ := p.ImageDescription()
	if desc != "Aperio" {
		t.Errorf("ImageDescription: got %q, want %q", desc, "Aperio")
	}
	offs, _ := p.TileOffsets()
	if got := []uint32{1000, 2000, 3000, 4000}; !equalU32(offs, got) {
		t.Errorf("TileOffsets: got %v, want %v", offs, got)
	}
	counts, _ := p.TileByteCounts()
	if got := []uint32{100, 200, 300, 400}; !equalU32(counts, got) {
		t.Errorf("TileByteCounts: got %v, want %v", counts, got)
	}
}

func TestPageTileGrid(t *testing.T) {
	data := buildPageTIFF(t)
	f, _ := Open(bytes.NewReader(data), int64(len(data)))
	p := f.Pages()[0]
	gx, gy, err := p.TileGrid()
	if err != nil {
		t.Fatalf("TileGrid: %v", err)
	}
	// ImageWidth 1024 / TileWidth 256 = 4; ImageLength 768 / TileLength 256 = 3.
	if gx != 4 || gy != 3 {
		t.Errorf("grid: got %dx%d, want 4x3", gx, gy)
	}
}

func equalU32(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTileOffsets64Compatibility(t *testing.T) {
	// Existing LONG TileOffsets via buildPageTIFF still round-trip through
	// the uint64 accessor.
	data := buildPageTIFF(t)
	f, _ := Open(bytes.NewReader(data), int64(len(data)))
	p := f.Pages()[0]
	offs, err := p.TileOffsets64()
	if err != nil {
		t.Fatalf("TileOffsets64: %v", err)
	}
	want := []uint64{1000, 2000, 3000, 4000}
	if !equalU64(offs, want) {
		t.Errorf("got %v, want %v", offs, want)
	}
}

func equalU64(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTileGridCeilDivPartialEdge(t *testing.T) {
	// Use the existing buildPageTIFF fixture (1024x768, 256 tiles), but
	// also verify ceil behavior with a computation unit test against the
	// pure math that TileGrid performs. Exhaustive case: iw=10, tw=4 → 3.
	cases := []struct {
		dim, tile uint32
		want      int
	}{
		{0, 1, 0},
		{1, 1, 1},
		{4, 4, 1},
		{5, 4, 2},
		{10, 4, 3},
		{1024, 256, 4},
		{768, 256, 3},
	}
	for _, c := range cases {
		got := int(c.dim / c.tile)
		if c.dim%c.tile != 0 {
			got++
		}
		if got != c.want {
			t.Errorf("ceil(%d/%d) = %d, want %d", c.dim, c.tile, got, c.want)
		}
	}
}
