package svs

import (
	"context"
	"fmt"
	"io"
	"iter"
	"math"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// tiledImage is the SVS Level implementation for tiled pages. v0.1 passes
// through the raw compressed TIFF tile bytes unmodified; no JPEG manipulation.
type tiledImage struct {
	index       int
	size        opentile.Size
	tileSize    opentile.Size
	grid        opentile.Size
	compression opentile.Compression
	mpp         opentile.SizeMm
	pyrIndex    int

	offsets []uint32
	counts  []uint32
	reader  io.ReaderAt

	cfg *opentile.Config
}

func newTiledImage(
	index int,
	p *tiff.Page,
	baseSize opentile.Size,
	baseMPP float64,
	r io.ReaderAt,
	cfg *opentile.Config,
) (*tiledImage, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("ImageLength missing")
	}
	tw, ok := p.TileWidth()
	if !ok || tw == 0 {
		return nil, fmt.Errorf("TileWidth missing or zero")
	}
	tl, ok := p.TileLength()
	if !ok || tl == 0 {
		return nil, fmt.Errorf("TileLength missing or zero")
	}
	gx, gy, err := p.TileGrid()
	if err != nil {
		return nil, err
	}
	offsets, err := p.TileOffsets()
	if err != nil {
		return nil, err
	}
	counts, err := p.TileByteCounts()
	if err != nil {
		return nil, err
	}
	if len(offsets) != len(counts) || len(offsets) != gx*gy {
		return nil, fmt.Errorf("tile table length mismatch: offsets=%d counts=%d grid=%dx%d", len(offsets), len(counts), gx, gy)
	}
	comp, _ := p.Compression()
	ocomp := mapCompression(comp)

	// Pyramid index: log2(baseSize.W / iw), rounded to nearest int.
	var pyr int
	if baseSize.W > 0 {
		pyr = int(math.Round(math.Log2(float64(baseSize.W) / float64(iw))))
		if pyr < 0 {
			pyr = 0
		}
	}

	scale := float64(1)
	if iw > 0 {
		scale = float64(baseSize.W) / float64(iw)
	}
	mpp := opentile.SizeMm{W: baseMPP * scale / 1000.0, H: baseMPP * scale / 1000.0}

	return &tiledImage{
		index:       index,
		size:        opentile.Size{W: int(iw), H: int(il)},
		tileSize:    opentile.Size{W: int(tw), H: int(tl)},
		grid:        opentile.Size{W: gx, H: gy},
		compression: ocomp,
		mpp:         mpp,
		pyrIndex:    pyr,
		offsets:     offsets,
		counts:      counts,
		reader:      r,
		cfg:         cfg,
	}, nil
}

// mapCompression translates TIFF compression codes into opentile.Compression.
func mapCompression(code uint32) opentile.Compression {
	switch code {
	case 1:
		return opentile.CompressionNone
	case 7:
		return opentile.CompressionJPEG
	case 33003, 33005:
		return opentile.CompressionJP2K
	default:
		return opentile.CompressionUnknown
	}
}

func (l *tiledImage) Index() int                        { return l.index }
func (l *tiledImage) PyramidIndex() int                 { return l.pyrIndex }
func (l *tiledImage) Size() opentile.Size               { return l.size }
func (l *tiledImage) TileSize() opentile.Size           { return l.tileSize }
func (l *tiledImage) Grid() opentile.Size               { return l.grid }
func (l *tiledImage) Compression() opentile.Compression { return l.compression }
func (l *tiledImage) MPP() opentile.SizeMm              { return l.mpp }
func (l *tiledImage) FocalPlane() float64               { return 0 }

func (l *tiledImage) indexOf(x, y int) (int, error) {
	if x < 0 || y < 0 || x >= l.grid.W || y >= l.grid.H {
		return 0, &opentile.TileError{Level: l.index, X: x, Y: y, Err: opentile.ErrTileOutOfBounds}
	}
	return y*l.grid.W + x, nil
}

// Tile returns the raw compressed tile bytes at (x, y).
func (l *tiledImage) Tile(x, y int) ([]byte, error) {
	idx, err := l.indexOf(x, y)
	if err != nil {
		return nil, err
	}
	length := l.counts[idx]
	if length == 0 {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: opentile.ErrCorruptTile}
	}
	off := int64(l.offsets[idx])
	buf := make([]byte, length)
	if _, err := l.reader.ReadAt(buf, off); err != nil {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: err}
	}
	return buf, nil
}

// TileReader returns an io.ReadCloser backed by an io.SectionReader so the
// tile bytes are streamed without buffering.
func (l *tiledImage) TileReader(x, y int) (io.ReadCloser, error) {
	idx, err := l.indexOf(x, y)
	if err != nil {
		return nil, err
	}
	length := l.counts[idx]
	if length == 0 {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: opentile.ErrCorruptTile}
	}
	sr := io.NewSectionReader(l.reader, int64(l.offsets[idx]), int64(length))
	return io.NopCloser(sr), nil
}

// Tiles iterates all tiles in row-major order.
func (l *tiledImage) Tiles(ctx context.Context) iter.Seq2[opentile.TilePos, opentile.TileResult] {
	return func(yield func(opentile.TilePos, opentile.TileResult) bool) {
		for y := 0; y < l.grid.H; y++ {
			for x := 0; x < l.grid.W; x++ {
				if err := ctx.Err(); err != nil {
					yield(opentile.TilePos{X: x, Y: y}, opentile.TileResult{Err: err})
					return
				}
				b, err := l.Tile(x, y)
				if !yield(opentile.TilePos{X: x, Y: y}, opentile.TileResult{Bytes: b, Err: err}) {
					return
				}
			}
		}
	}
}
