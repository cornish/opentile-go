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
