package opentile

import (
	"context"
	"image"
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

	// TileOverlap returns the pixel overlap between adjacent tiles at this level.
	// Tile (c, r) is positioned in image-space at
	//   (c · (TileSize().X - TileOverlap().X),
	//    r · (TileSize().Y - TileOverlap().Y)).
	// In the overlap region, tiles further along the row/column overwrite earlier
	// tiles (no blending). Returns image.Point{} for non-overlapping levels and
	// non-BIF formats.
	TileOverlap() image.Point

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

// Image represents one main pyramid in a Tiler. Single-image formats
// (SVS, NDPI, Philips) expose exactly one Image; OME-TIFF can expose
// multiple — one per OME <Image> element that isn't classified as
// macro / label / thumbnail.
//
// Within an Image the Levels are ordered from highest resolution
// (Index 0 = baseline) downwards.
//
// Added in v0.6 alongside Tiler.Images(). The legacy Tiler.Levels() /
// Tiler.Level(i) shortcut accessors continue to work, delegating to
// Images()[0].
type Image interface {
	// Index is the 0-based document-order index of this Image within
	// the Tiler. Always 0 for single-image formats; 0..N-1 for
	// multi-image OME.
	Index() int
	// Name is the format-specific name for this Image — for OME TIFF,
	// the <Image Name="..."> attribute (typically empty for main
	// pyramids; macro / label / thumbnail are routed to AssociatedImage
	// rather than Image). Empty string for non-OME formats.
	Name() string
	// Levels returns the pyramid levels from highest to lowest
	// resolution. Always returns a fresh slice; callers may mutate the
	// slice header without affecting the Image's internal state.
	Levels() []Level
	// Level returns the level at the given index, or
	// ErrLevelOutOfRange if i is out of bounds.
	Level(i int) (Level, error)
	// MPP returns the base-level microns/pixel for this Image. Zero
	// SizeMm when unknown.
	MPP() SizeMm
}

// SingleImage is the one-element Image wrapper used by single-pyramid
// formats (SVS, NDPI, Philips) to satisfy Tiler.Images(). It holds a
// fixed level list; Index() is always 0, Name() always empty, and
// MPP() returns the base level's MPP() (or the zero SizeMm when the
// level list is empty).
//
// Multi-image formats (OME-TIFF) implement opentile.Image directly
// rather than reusing SingleImage.
type SingleImage struct {
	levels []Level
}

// NewSingleImage returns an Image wrapping the supplied level slice.
// The slice header is copied; underlying Level pointers are shared.
func NewSingleImage(levels []Level) *SingleImage {
	cp := make([]Level, len(levels))
	copy(cp, levels)
	return &SingleImage{levels: cp}
}

// Index always returns 0 for SingleImage.
func (s *SingleImage) Index() int { return 0 }

// Name always returns "" for SingleImage.
func (s *SingleImage) Name() string { return "" }

// Levels returns a fresh copy of the wrapped level slice.
func (s *SingleImage) Levels() []Level {
	out := make([]Level, len(s.levels))
	copy(out, s.levels)
	return out
}

// Level returns the level at i, or ErrLevelOutOfRange if i is out
// of bounds.
func (s *SingleImage) Level(i int) (Level, error) {
	if i < 0 || i >= len(s.levels) {
		return nil, ErrLevelOutOfRange
	}
	return s.levels[i], nil
}

// MPP returns the base level's MPP, or the zero SizeMm if there are
// no levels.
func (s *SingleImage) MPP() SizeMm {
	if len(s.levels) == 0 {
		return SizeMm{}
	}
	return s.levels[0].MPP()
}

// TilePos is a (column, row) pair returned by Level.Tiles.
type TilePos struct{ X, Y int }

// TileResult carries the yield from Level.Tiles.
type TileResult struct {
	Bytes []byte
	Err   error
}
