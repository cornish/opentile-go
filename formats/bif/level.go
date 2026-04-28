package bif

import (
	"context"
	"fmt"
	"image"
	"io"
	"iter"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/bifxml"
	"github.com/cornish/opentile-go/internal/tiff"
)

// levelImpl is the BIF Level implementation. One per pyramid IFD.
//
// v0.7 always returns the raw compressed JPEG tile bytes as stored
// in the source TIFF — no JPEGTables splice (T15 wires that), no
// decode, no pixel-space crop. The byte-passthrough hot path is
// preserved.
//
// Tile addressing: callers use image-space row-major (col, row);
// internally the (col, row) is remapped via imageToSerpentine to find
// the entry in TileOffsets/TileByteCounts. The remap is cheap (a
// few ops) — no per-tile XMP lookup required.
type levelImpl struct {
	index       int                 // 0-based level index in the Image's Levels slice
	pyrIndex    int                 // parsed `level=N` value from ImageDescription
	size        opentile.Size       // base ImageWidth × ImageLength of this pyramid level
	tileSize    opentile.Size       // TileWidth × TileLength
	grid        opentile.Size       // tile grid dimensions (cols × rows)
	compression opentile.Compression
	mpp         opentile.SizeMm
	tileOverlap image.Point // non-zero only on level 0 of overlapping spec-compliant slides

	offsets []uint64 // TileOffsets, in serpentine storage order
	counts  []uint64 // TileByteCounts, in serpentine storage order

	reader io.ReaderAt // for SectionReader-based streaming
}

// newLevelImpl constructs a levelImpl from a classified IFD. The
// EncodeInfo (parsed from the level-0 IFD's XMP) supplies tile
// overlap data — non-zero only when this is the level-0 IFD AND the
// XMP carried meaningful TileJointInfo OverlapX/OverlapY values.
func newLevelImpl(
	index int,
	c classifiedIFD,
	baseMPP float64,
	encodeInfo *bifxml.EncodeInfo,
	reader io.ReaderAt,
) (*levelImpl, error) {
	p := c.Page
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("bif level=%d: ImageWidth missing", c.Level)
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("bif level=%d: ImageLength missing", c.Level)
	}
	tw, ok := p.TileWidth()
	if !ok || tw == 0 {
		return nil, fmt.Errorf("bif level=%d: TileWidth missing or zero", c.Level)
	}
	tl, ok := p.TileLength()
	if !ok || tl == 0 {
		return nil, fmt.Errorf("bif level=%d: TileLength missing or zero", c.Level)
	}
	cols, rows, err := p.TileGrid()
	if err != nil {
		return nil, fmt.Errorf("bif level=%d: TileGrid: %w", c.Level, err)
	}
	offsets, err := p.TileOffsets64()
	if err != nil {
		return nil, fmt.Errorf("bif level=%d: TileOffsets: %w", c.Level, err)
	}
	counts, err := p.TileByteCounts64()
	if err != nil {
		return nil, fmt.Errorf("bif level=%d: TileByteCounts: %w", c.Level, err)
	}
	if len(offsets) != len(counts) || len(offsets) != cols*rows {
		return nil, fmt.Errorf("bif level=%d: tile table length mismatch: offsets=%d counts=%d grid=%dx%d", c.Level, len(offsets), len(counts), cols, rows)
	}

	comp, _ := p.Compression()
	ocomp := tiffCompressionToOpentile(comp)

	// Per-level MPP: base ScanRes (microns/pixel at level 0) doubled
	// per pyramid step. SizeMm is in millimeters.
	levelMPPMicrons := baseMPP * float64(int(1)<<c.Level)
	mpp := opentile.SizeMm{W: levelMPPMicrons / 1000.0, H: levelMPPMicrons / 1000.0}

	// TileOverlap is the per-level tile step deficit. Only the
	// level-0 IFD's EncodeInfo carries TileJointInfo entries; we
	// collapse them into a single weighted-average value per spec
	// §8 (matches openslide). Pyramid levels 1+ are non-overlapping
	// per the whitepaper, so they always return image.Point{}.
	var tileOverlap image.Point
	if c.Level == 0 && encodeInfo != nil {
		tileOverlap = weightedAverageOverlap(encodeInfo)
	}

	return &levelImpl{
		index:       index,
		pyrIndex:    c.Level,
		size:        opentile.Size{W: int(iw), H: int(il)},
		tileSize:    opentile.Size{W: int(tw), H: int(tl)},
		grid:        opentile.Size{W: cols, H: rows},
		compression: ocomp,
		mpp:         mpp,
		tileOverlap: tileOverlap,
		offsets:     offsets,
		counts:      counts,
		reader:      reader,
	}, nil
}

// weightedAverageOverlap collapses EncodeInfo's per-tile-pair
// `<TileJointInfo>` entries into a single (X, Y) image.Point using
// pixel-count weighting — matches openslide's
// `tile_advance_x / tile_advance_y` computation. Returns image.Point{}
// if there are no joint entries.
//
// The whitepaper says DP 200 only ever produces horizontal overlap
// (OverlapY == 0); we don't enforce that here, just report the data.
// Both local fixtures record OverlapX = OverlapY = 0, so this fold
// returns {0, 0} on real data and the non-zero path is exercised
// only by synthetic-XMP unit tests.
func weightedAverageOverlap(ei *bifxml.EncodeInfo) image.Point {
	var sumX, sumY, count int
	for _, info := range ei.ImageInfos {
		for _, j := range info.Joints {
			sumX += j.OverlapX
			sumY += j.OverlapY
			count++
		}
	}
	if count == 0 {
		return image.Point{}
	}
	return image.Point{X: sumX / count, Y: sumY / count}
}

func (l *levelImpl) Index() int                        { return l.index }
func (l *levelImpl) PyramidIndex() int                 { return l.pyrIndex }
func (l *levelImpl) Size() opentile.Size               { return l.size }
func (l *levelImpl) TileSize() opentile.Size           { return l.tileSize }
func (l *levelImpl) Grid() opentile.Size               { return l.grid }
func (l *levelImpl) Compression() opentile.Compression { return l.compression }
func (l *levelImpl) MPP() opentile.SizeMm              { return l.mpp }
func (l *levelImpl) FocalPlane() float64               { return 0 }
func (l *levelImpl) TileOverlap() image.Point          { return l.tileOverlap }

// indexOf validates (col, row) is within the tile grid and returns
// the serpentine index into offsets/counts.
func (l *levelImpl) indexOf(col, row int) (int, error) {
	if col < 0 || row < 0 || col >= l.grid.W || row >= l.grid.H {
		return 0, &opentile.TileError{Level: l.index, X: col, Y: row, Err: opentile.ErrTileOutOfBounds}
	}
	idx := imageToSerpentine(col, row, l.grid.W, l.grid.H)
	if idx < 0 || idx >= len(l.offsets) {
		// Defensive — imageToSerpentine should never return -1 for in-bounds (col, row).
		return 0, &opentile.TileError{Level: l.index, X: col, Y: row, Err: opentile.ErrTileOutOfBounds}
	}
	return idx, nil
}

// Tile returns the raw compressed tile bytes at (col, row) in
// image-space. Internally remapped to TileOffsets storage order via
// imageToSerpentine.
//
// v0.7 returns the source TIFF bytes verbatim — no JPEGTables splice
// (T15) and no blank-tile fill on empty entries (T14). A
// `TileByteCounts[i] == 0` entry currently yields ErrCorruptTile;
// T14 will swap that branch for blankTile().
func (l *levelImpl) Tile(col, row int) ([]byte, error) {
	idx, err := l.indexOf(col, row)
	if err != nil {
		return nil, err
	}
	length := l.counts[idx]
	if length == 0 {
		return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: opentile.ErrCorruptTile}
	}
	off := int64(l.offsets[idx])
	buf := make([]byte, length)
	if err := tiff.ReadAtFull(l.reader, buf, off); err != nil {
		return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: err}
	}
	return buf, nil
}

// TileReader returns a streaming reader over the tile at (col, row).
// Until T15 splices JPEGTables, this is a straight io.SectionReader
// over the TIFF — zero-copy.
func (l *levelImpl) TileReader(col, row int) (io.ReadCloser, error) {
	idx, err := l.indexOf(col, row)
	if err != nil {
		return nil, err
	}
	length := l.counts[idx]
	if length == 0 {
		return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: opentile.ErrCorruptTile}
	}
	off := int64(l.offsets[idx])
	return io.NopCloser(io.NewSectionReader(l.reader, off, int64(length))), nil
}

// Tiles iterates every tile position in image-space row-major order.
// Serial — callers parallelise on top of Tile(c, r) if needed.
func (l *levelImpl) Tiles(ctx context.Context) iter.Seq2[opentile.TilePos, opentile.TileResult] {
	return func(yield func(opentile.TilePos, opentile.TileResult) bool) {
		for r := 0; r < l.grid.H; r++ {
			for c := 0; c < l.grid.W; c++ {
				if ctx.Err() != nil {
					return
				}
				b, err := l.Tile(c, r)
				if !yield(opentile.TilePos{X: c, Y: r}, opentile.TileResult{Bytes: b, Err: err}) {
					return
				}
			}
		}
	}
}

// tiffCompressionToOpentile maps the TIFF Compression tag value to
// opentile's Compression enum. Mirrors `formats/svs/tiled.go::tiffCompressionToOpentile`
// (BIF only ever uses JPEG=7 on pyramid IFDs per the whitepaper, but
// we include the small switch for completeness and consistency with
// other format packages).
func tiffCompressionToOpentile(c uint32) opentile.Compression {
	switch c {
	case 7:
		return opentile.CompressionJPEG
	case 1:
		return opentile.CompressionNone
	case 5:
		return opentile.CompressionLZW
	default:
		return opentile.CompressionUnknown
	}
}

// _ static interface assertion — fail at compile time if levelImpl
// drifts away from the Level contract. T8 added TileOverlap; if the
// project evolves Level further this catches the breakage early.
var _ opentile.Level = (*levelImpl)(nil)
