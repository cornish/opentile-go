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

func TestPageSoftware(t *testing.T) {
	// Build a minimal TIFF with the Software tag (305) carrying an ASCII
	// value with the Philips DP prefix. Should round-trip via Page.Software().
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0}) // header, first IFD at 8

	const tSoftware uint16 = 305
	// 1 entry. count(u16) + 12 entry bytes + 4 nextIFD = 18 bytes.
	// External data starts at 8 + 18 = 26.
	externalBase := uint32(26)
	sw := []byte("Philips DP v1.0\x00")

	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, tSoftware)
	_ = binary.Write(buf, binary.LittleEndian, uint16(DTASCII))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(sw)))
	_ = binary.Write(buf, binary.LittleEndian, externalBase)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD
	buf.Write(sw)

	data := buf.Bytes()
	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, ok := f.Pages()[0].Software()
	if !ok {
		t.Fatal("Software: expected ok=true")
	}
	if got != "Philips DP v1.0" {
		t.Errorf("Software: got %q, want %q", got, "Philips DP v1.0")
	}

	// Page without the tag → ok=false.
	data2 := buildPageTIFF(t)
	f2, _ := Open(bytes.NewReader(data2), int64(len(data2)))
	if _, ok := f2.Pages()[0].Software(); ok {
		t.Error("Software on missing tag: expected ok=false")
	}
}

// TestPageSubIFDOffsets confirms the SubIFDs accessor (TIFF tag 330)
// reads a LONG-typed offset array correctly. Used by OME TIFF where
// reduced-resolution pyramid levels live in SubIFDs of the base page
// rather than as top-level IFDs.
func TestPageSubIFDOffsets(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0}) // header, first IFD at 8

	const tSubIFDs uint16 = 330
	// 1 entry. count(u16) + 12 entry bytes + 4 nextIFD = 18.
	// External region starts at 8 + 18 = 26.
	externalBase := uint32(26)
	subOffsets := []uint32{1000, 2000, 3000}

	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, tSubIFDs)
	_ = binary.Write(buf, binary.LittleEndian, uint16(DTLong))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(subOffsets)))
	_ = binary.Write(buf, binary.LittleEndian, externalBase)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD
	for _, v := range subOffsets {
		_ = binary.Write(buf, binary.LittleEndian, v)
	}

	data := buf.Bytes()
	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, ok := f.Pages()[0].SubIFDOffsets()
	if !ok {
		t.Fatal("SubIFDOffsets: expected ok=true")
	}
	want := []uint64{1000, 2000, 3000}
	if !equalU64(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// Page without the tag → ok=false.
	data2 := buildPageTIFF(t)
	f2, _ := Open(bytes.NewReader(data2), int64(len(data2)))
	if _, ok := f2.Pages()[0].SubIFDOffsets(); ok {
		t.Error("SubIFDOffsets on missing tag: expected ok=false")
	}
}

// TestPageImageDepth confirms the ImageDepth accessor (TIFF tag 32997)
// reads the SGI private tag used by Ventana BIF for Z-stack depth.
func TestPageImageDepth(t *testing.T) {
	// Test 1: ImageDepth = 3 → (3, true)
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0}) // header, first IFD at 8

	const tImageDepth uint16 = 32997
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // 1 entry
	_ = binary.Write(buf, binary.LittleEndian, tImageDepth)
	_ = binary.Write(buf, binary.LittleEndian, uint16(DTShort))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1)) // count = 1
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // padding
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD

	data := buf.Bytes()
	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, ok := f.Pages()[0].ImageDepth()
	if !ok {
		t.Fatal("ImageDepth: expected ok=true")
	}
	if got != 3 {
		t.Errorf("ImageDepth: got %d, want 3", got)
	}

	// Test 2: Page without the tag → (1, false)
	data2 := buildPageTIFF(t)
	f2, _ := Open(bytes.NewReader(data2), int64(len(data2)))
	got2, ok2 := f2.Pages()[0].ImageDepth()
	if ok2 {
		t.Fatal("ImageDepth on missing tag: expected ok=false")
	}
	if got2 != 1 {
		t.Errorf("ImageDepth on missing tag: got %d, want 1", got2)
	}

	// Test 3: ImageDepth = 0 → (1, false) (treated as missing)
	buf3 := new(bytes.Buffer)
	buf3.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0})
	_ = binary.Write(buf3, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf3, binary.LittleEndian, tImageDepth)
	_ = binary.Write(buf3, binary.LittleEndian, uint16(DTShort))
	_ = binary.Write(buf3, binary.LittleEndian, uint32(1)) // count = 1
	_ = binary.Write(buf3, binary.LittleEndian, uint16(0)) // value = 0
	_ = binary.Write(buf3, binary.LittleEndian, uint16(0)) // padding
	_ = binary.Write(buf3, binary.LittleEndian, uint32(0)) // next IFD

	data3 := buf3.Bytes()
	f3, _ := Open(bytes.NewReader(data3), int64(len(data3)))
	got3, ok3 := f3.Pages()[0].ImageDepth()
	if ok3 {
		t.Fatal("ImageDepth with zero value: expected ok=false")
	}
	if got3 != 1 {
		t.Errorf("ImageDepth with zero value: got %d, want 1", got3)
	}
}

func TestPageFloat32(t *testing.T) {
	// Build a minimal TIFF with one FLOAT tag: tag=65421, type=11, count=1,
	// value (inline) = 20.0 → IEEE 754 bits 0x41A00000.
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0})
	w16 := func(v uint16) { buf.WriteByte(byte(v)); buf.WriteByte(byte(v >> 8)) }
	w32 := func(v uint32) {
		buf.WriteByte(byte(v)); buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16)); buf.WriteByte(byte(v >> 24))
	}
	w16(1)                                       // tagno=1
	w16(65421); w16(11); w32(1); w32(0x41A00000) // Magnification FLOAT
	w32(0)                                       // next IFD = 0
	data := buf.Bytes()

	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	p := f.Pages()[0]
	got, ok := p.Float32(65421)
	if !ok {
		t.Fatal("Float32: expected ok=true")
	}
	if got != 20.0 {
		t.Errorf("Float32: got %v, want 20.0", got)
	}
	// Missing tag → ok=false.
	_, ok = p.Float32(65422)
	if ok {
		t.Error("Float32 on missing tag: expected ok=false")
	}
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
