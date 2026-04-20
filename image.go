package opentile

import (
	"context"
	"io"
	"iter"
)

// Level is a single resolution in a pyramidal WSI.
//
// Tile and TileReader are safe for concurrent use from multiple goroutines,
// provided the io.ReaderAt supplied to Open is also safe for concurrent use.
// (stdlib *os.File satisfies this.)
type Level interface {
	Index() int
	PyramidIndex() int
	Size() Size
	TileSize() Size
	Grid() Size
	Compression() Compression
	MPP() SizeMm
	FocalPlane() float64

	// Tile returns the raw compressed tile bytes at (x, y) as stored in the
	// source TIFF.
	Tile(x, y int) ([]byte, error)

	// TileReader returns a streaming reader for the tile at (x, y). Callers
	// should Close the returned ReadCloser.
	TileReader(x, y int) (io.ReadCloser, error)

	// Tiles iterates every tile position in row-major order. Callers that need
	// parallelism goroutine on top of Tile(x, y); Tiles itself is serial.
	Tiles(ctx context.Context) iter.Seq2[TilePos, TileResult]
}

// AssociatedImage is a non-pyramidal slide-level image (label, overview,
// thumbnail). v0.1 returns an empty slice from Tiler.Associated().
type AssociatedImage interface {
	Kind() string
	Size() Size
	Compression() Compression
	Bytes() ([]byte, error)
}

// TilePos is a (column, row) pair returned by Level.Tiles.
type TilePos struct{ X, Y int }

// TileResult carries the yield from Level.Tiles.
type TileResult struct {
	Bytes []byte
	Err   error
}
