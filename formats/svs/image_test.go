package svs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// buildSVSTIFF builds a TIFF with one tiled page carrying tileCount*tileCount
// synthetic tile payloads (each unique). Returns (bytes, tileBytes[idx]).
// The ImageDescription starts with "Aperio" so the SVS factory accepts it.
func buildSVSTIFF(t *testing.T, tileW, tileH, tilesX, tilesY int, extraDesc string) (data []byte, tiles [][]byte) {
	t.Helper()
	// Build tiles: each is a unique byte pattern of length 32.
	nTiles := tilesX * tilesY
	tiles = make([][]byte, nTiles)
	for i := 0; i < nTiles; i++ {
		buf := make([]byte, 32)
		for j := range buf {
			buf[j] = byte(i*7 + j) // arbitrary deterministic
		}
		tiles[i] = buf
	}
	desc := "Aperio Test\n"
	if extraDesc != "" {
		desc += extraDesc
	}
	descBytes := append([]byte(desc), 0)

	// Layout: Header (8) + IFD at 8 + external data after.
	// IFD entries: ImageWidth, ImageLength, Compression, Photometric,
	// ImageDescription, TileWidth, TileLength, TileOffsets, TileByteCounts = 9
	// IFD size = 2 + 9*12 + 4 = 114
	ifdStart := uint32(8)
	extStart := ifdStart + 114

	descOff := extStart
	extAfterDesc := descOff + uint32(len(descBytes))

	tileBCOff := extAfterDesc
	extAfterBC := tileBCOff + uint32(4*nTiles)

	tileOffOff := extAfterBC
	extAfterTO := tileOffOff + uint32(4*nTiles)

	// Tile data offsets: pack consecutively starting at extAfterTO.
	tileOffsets := make([]uint32, nTiles)
	off := extAfterTO
	for i := range tiles {
		tileOffsets[i] = off
		off += uint32(len(tiles[i]))
	}

	buf := new(bytes.Buffer)
	w16 := func(v uint16) { buf.WriteByte(byte(v)); buf.WriteByte(byte(v >> 8)) }
	w32 := func(v uint32) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0})
	w16(9)
	entry := func(tag, typ uint16, count, voc uint32) {
		w16(tag); w16(typ); w32(count); w32(voc)
	}
	entry(256, 3, 1, uint32(tileW*tilesX)) // ImageWidth
	entry(257, 3, 1, uint32(tileH*tilesY)) // ImageLength
	entry(259, 3, 1, 7)                    // Compression = JPEG
	entry(262, 3, 1, 6)                    // Photometric = YCbCr
	entry(270, 2, uint32(len(descBytes)), descOff)
	entry(322, 3, 1, uint32(tileW))
	entry(323, 3, 1, uint32(tileH))
	entry(324, 4, uint32(nTiles), tileOffOff)
	entry(325, 4, uint32(nTiles), tileBCOff)
	w32(0) // next IFD

	// External region
	buf.Write(descBytes)
	// Write TileByteCounts
	for _, tb := range tiles {
		w32(uint32(len(tb)))
	}
	// Write TileOffsets
	for _, o := range tileOffsets {
		w32(o)
	}
	// Finally, write tile payloads in the same order.
	for _, tb := range tiles {
		buf.Write(tb)
	}
	return buf.Bytes(), tiles
}

func TestSvsTilerOpenAndLevel(t *testing.T) {
	data, tiles := buildSVSTIFF(t, 16, 16, 3, 2, "AppMag = 20|MPP = 0.5")
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	cfg := opentile.NewTestConfig(opentile.Size{}, opentile.CorruptTileError)
	tiler, err := New().Open(f, cfg)
	if err != nil {
		t.Fatalf("svs.New().Open: %v", err)
	}
	defer tiler.Close()

	levels := tiler.Levels()
	if len(levels) != 1 {
		t.Fatalf("levels: got %d, want 1", len(levels))
	}
	lvl, err := tiler.Level(0)
	if err != nil {
		t.Fatalf("Level(0): %v", err)
	}
	if got := lvl.TileSize(); got.W != 16 || got.H != 16 {
		t.Errorf("TileSize: got %v, want 16x16", got)
	}
	if got := lvl.Grid(); got.W != 3 || got.H != 2 {
		t.Errorf("Grid: got %v, want 3x2", got)
	}
	// Tile (0,0) → first tile payload
	b, err := lvl.Tile(0, 0)
	if err != nil {
		t.Fatalf("Tile(0,0): %v", err)
	}
	if !bytes.Equal(b, tiles[0]) {
		t.Fatalf("Tile(0,0) bytes mismatch")
	}
	// Tile (2,1) → last tile (index 5)
	b, err = lvl.Tile(2, 1)
	if err != nil {
		t.Fatalf("Tile(2,1): %v", err)
	}
	if !bytes.Equal(b, tiles[5]) {
		t.Fatalf("Tile(2,1) bytes mismatch")
	}
}

func TestSvsLevelTileOutOfBounds(t *testing.T) {
	data, _ := buildSVSTIFF(t, 16, 16, 2, 2, "")
	f, _ := tiff.Open(bytes.NewReader(data), int64(len(data)))
	cfg := opentile.NewTestConfig(opentile.Size{}, opentile.CorruptTileError)
	tiler, _ := New().Open(f, cfg)
	lvl, _ := tiler.Level(0)
	_, err := lvl.Tile(99, 99)
	if !errors.Is(err, opentile.ErrTileOutOfBounds) {
		t.Fatalf("expected ErrTileOutOfBounds, got %v", err)
	}
	var te *opentile.TileError
	if !errors.As(err, &te) {
		t.Fatal("expected TileError wrapping")
	}
	if te.X != 99 || te.Y != 99 {
		t.Errorf("TileError coords: got %d,%d", te.X, te.Y)
	}
}

func TestSvsLevelTilesIterator(t *testing.T) {
	data, tiles := buildSVSTIFF(t, 16, 16, 2, 2, "")
	f, _ := tiff.Open(bytes.NewReader(data), int64(len(data)))
	cfg := opentile.NewTestConfig(opentile.Size{}, opentile.CorruptTileError)
	tiler, _ := New().Open(f, cfg)
	lvl, _ := tiler.Level(0)

	ctx := context.Background()
	count := 0
	for pos, res := range lvl.Tiles(ctx) {
		if res.Err != nil {
			t.Fatalf("Tiles err at %v: %v", pos, res.Err)
		}
		idx := pos.Y*2 + pos.X
		if !bytes.Equal(res.Bytes, tiles[idx]) {
			t.Errorf("mismatch at %v", pos)
		}
		count++
	}
	if count != 4 {
		t.Errorf("count: got %d, want 4", count)
	}
}

func TestSvsLevelTileReader(t *testing.T) {
	data, tiles := buildSVSTIFF(t, 16, 16, 2, 2, "")
	f, _ := tiff.Open(bytes.NewReader(data), int64(len(data)))
	cfg := opentile.NewTestConfig(opentile.Size{}, opentile.CorruptTileError)
	tiler, _ := New().Open(f, cfg)
	lvl, _ := tiler.Level(0)
	rc, err := lvl.TileReader(1, 1)
	if err != nil {
		t.Fatalf("TileReader: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, tiles[3]) {
		t.Fatalf("TileReader(1,1) bytes mismatch")
	}
}

// truncatingReaderAt wraps an io.ReaderAt and returns (n, io.EOF) when a read
// lands exactly at the reader's end, even if all requested bytes were delivered.
// Mirrors bytes.Reader.ReadAt semantics on boundary reads.
type truncatingReaderAt struct {
	r    io.ReaderAt
	size int64
}

func (t *truncatingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := t.r.ReadAt(p, off)
	if err == nil && off+int64(n) == t.size {
		return n, io.EOF
	}
	return n, err
}

func TestSvsLevelTileBenignEOF(t *testing.T) {
	// Use a 2×1 grid so TileOffsets/TileByteCounts have count=2 and are stored
	// externally (2*4=8 > 4 bytes inline limit). The second tile (x=1,y=0)
	// occupies the very last bytes of the file, so the truncatingReaderAt wrapper
	// surfaces (n=len(buf), io.EOF) on that read, exercising the benign-EOF path.
	data, tiles := buildSVSTIFF(t, 16, 16, 2, 1, "")
	// Wrap the reader so the final boundary read surfaces (n, io.EOF).
	base := bytes.NewReader(data)
	trunc := &truncatingReaderAt{r: base, size: int64(len(data))}
	f, err := tiff.Open(trunc, int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	cfg := opentile.NewTestConfig(opentile.Size{}, opentile.CorruptTileError)
	tiler, err := New().Open(f, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tiler.Close()
	lvl, _ := tiler.Level(0)
	// Tile (1,0) is the last tile and lands exactly at end-of-file.
	got, err := lvl.Tile(1, 0)
	if err != nil {
		t.Fatalf("Tile: unexpected error (likely benign-EOF bug): %v", err)
	}
	if !bytes.Equal(got, tiles[1]) {
		t.Fatal("tile bytes mismatch on benign-EOF path")
	}
}

func TestMetadataOfExtractsAperioExtras(t *testing.T) {
	data, _ := buildSVSTIFF(t, 16, 16, 1, 1, "AppMag = 40|MPP = 0.25|Filename = slide-x")
	f, _ := tiff.Open(bytes.NewReader(data), int64(len(data)))
	cfg := opentile.NewTestConfig(opentile.Size{}, opentile.CorruptTileError)
	tiler, err := New().Open(f, cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tiler.Close()

	md, ok := MetadataOf(tiler)
	if !ok {
		t.Fatal("MetadataOf: expected ok=true for SVS tiler")
	}
	if md.MPP != 0.25 {
		t.Errorf("MPP: got %v, want 0.25", md.MPP)
	}
	if md.Filename != "slide-x" {
		t.Errorf("Filename: got %q, want slide-x", md.Filename)
	}
	if md.Magnification != 40 {
		t.Errorf("Magnification: got %v, want 40", md.Magnification)
	}
}

func TestMetadataOfRejectsNonSVSTiler(t *testing.T) {
	// An arbitrary opentile.Tiler that is not *svs.tiler — use a zero-value
	// fakeTiler implementation.
	fake := &fakeNonSVSTiler{}
	_, ok := MetadataOf(fake)
	if ok {
		t.Fatal("MetadataOf: expected ok=false for non-SVS Tiler")
	}
}

type fakeNonSVSTiler struct{}

func (f *fakeNonSVSTiler) Format() opentile.Format                { return opentile.Format("fake") }
func (f *fakeNonSVSTiler) Levels() []opentile.Level               { return nil }
func (f *fakeNonSVSTiler) Level(i int) (opentile.Level, error)    { return nil, opentile.ErrLevelOutOfRange }
func (f *fakeNonSVSTiler) Associated() []opentile.AssociatedImage { return nil }
func (f *fakeNonSVSTiler) Metadata() opentile.Metadata            { return opentile.Metadata{} }
func (f *fakeNonSVSTiler) ICCProfile() []byte                     { return nil }
func (f *fakeNonSVSTiler) Close() error                           { return nil }
