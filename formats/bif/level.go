package bif

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"iter"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/bifxml"
	"github.com/cornish/opentile-go/internal/jpeg"
	"github.com/cornish/opentile-go/internal/tiff"
)

// bytesReader is a small adapter so blank-tile TileReader can return
// a bytes.Reader-backed io.Reader without allocating per-call when
// the same blank-tile cache entry is requested repeatedly.
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

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

	// jpegTables is TIFF tag 347 (JPEGTables) if present. Older
	// BIFs embed full JPEG headers in each tile (jpegTables nil);
	// newer BIFs (and OS-1) carry shared tables here that must be
	// spliced into each abbreviated tile scan via jpeg.InsertTables
	// before the bytes can decode. BIF is YCbCr — no APP14 RGB
	// colorspace-fix marker needed (unlike SVS).
	jpegTables []byte

	// scanWhitePoint is the white-fill luminance for empty (unscanned)
	// tiles per spec §"AOI Positions". 0..255. Spec-compliant slides
	// inherit this from <iScan>/@ScanWhitePoint; legacy iScan and any
	// slide where the attribute is absent fall back to 255 (true white).
	scanWhitePoint uint8

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
	scanWhitePoint uint8,
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

	// Tag 347 (JPEGTables): optional per spec. Newer BIFs share
	// DQT/DHT here so abbreviated per-tile scans can decode; older
	// BIFs embed everything per-tile. Both arrangements are valid
	// within the spec and both must be supported.
	var jpegTables []byte
	if ocomp == opentile.CompressionJPEG {
		if tb, ok := p.JPEGTables(); ok {
			jpegTables = tb
		}
	}

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
		index:          index,
		pyrIndex:       c.Level,
		size:           opentile.Size{W: int(iw), H: int(il)},
		tileSize:       opentile.Size{W: int(tw), H: int(tl)},
		grid:           opentile.Size{W: cols, H: rows},
		compression:    ocomp,
		mpp:            mpp,
		tileOverlap:    tileOverlap,
		offsets:        offsets,
		counts:         counts,
		jpegTables:     jpegTables,
		scanWhitePoint: scanWhitePoint,
		reader:         reader,
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

// Tile returns the compressed tile bytes at (col, row) in
// image-space — a standalone valid JPEG (or, for theoretical
// non-JPEG BIF dialects, the raw codestream). Internally remapped
// to TileOffsets storage order via imageToSerpentine.
//
//   - Empty tiles (`TileOffsets[i] == 0 && TileByteCounts[i] == 0`,
//     per BIF whitepaper §"AOI Positions") return a synthesised JPEG
//     filled with the slide's `ScanWhitePoint` luminance — see
//     blankTile.
//   - When the IFD carries shared JPEGTables (tag 347), the per-tile
//     abbreviated scan bytes have DQT/DHT spliced before SOS via
//     jpeg.InsertTables so the result decodes standalone. BIF is
//     YCbCr — no APP14 RGB colorspace-fix is needed (unlike SVS).
//   - When JPEGTables is absent, the tile bytes carry their own
//     SOI/DQT/DHT/SOF and are returned verbatim.
func (l *levelImpl) Tile(col, row int) ([]byte, error) {
	idx, err := l.indexOf(col, row)
	if err != nil {
		return nil, err
	}
	if l.isEmpty(idx) {
		b, err := blankTile(l.tileSize.W, l.tileSize.H, l.scanWhitePoint)
		if err != nil {
			return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: err}
		}
		return b, nil
	}
	length := l.counts[idx]
	off := int64(l.offsets[idx])
	buf := make([]byte, length)
	if err := tiff.ReadAtFull(l.reader, buf, off); err != nil {
		return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: err}
	}
	if l.compression == opentile.CompressionJPEG && len(l.jpegTables) > 0 {
		out, err := jpeg.InsertTables(buf, l.jpegTables)
		if err != nil {
			return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: err}
		}
		return out, nil
	}
	return buf, nil
}

// TileReader returns a streaming reader over the tile at (col, row).
//
//   - Empty entries: a bytes.Reader over the cached blank tile.
//   - JPEG entries with shared tables: the splice produces a buffer
//     that doesn't correspond to a contiguous file region, so we
//     return a bytes.Reader over the spliced output. Streaming
//     buys nothing here.
//   - Other JPEG entries: a zero-copy io.SectionReader over the
//     source TIFF.
func (l *levelImpl) TileReader(col, row int) (io.ReadCloser, error) {
	idx, err := l.indexOf(col, row)
	if err != nil {
		return nil, err
	}
	if l.isEmpty(idx) {
		b, err := blankTile(l.tileSize.W, l.tileSize.H, l.scanWhitePoint)
		if err != nil {
			return nil, &opentile.TileError{Level: l.index, X: col, Y: row, Err: err}
		}
		return io.NopCloser(bytesReader(b)), nil
	}
	if l.compression == opentile.CompressionJPEG && len(l.jpegTables) > 0 {
		b, err := l.Tile(col, row)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytesReader(b)), nil
	}
	length := l.counts[idx]
	off := int64(l.offsets[idx])
	return io.NopCloser(io.NewSectionReader(l.reader, off, int64(length))), nil
}

// isEmpty reports whether the tile at TileOffsets index idx is the
// spec-defined empty marker (offset == 0 AND bytecount == 0).
func (l *levelImpl) isEmpty(idx int) bool {
	return l.offsets[idx] == 0 && l.counts[idx] == 0
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
