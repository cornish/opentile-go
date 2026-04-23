package ndpi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/jpeg"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// stripedImage is an NDPI Level backed by a page of 8-pixel-tall horizontal
// stripes. Each output Tile is assembled from multiple native stripes via
// pure-Go JPEG marker concatenation.
type stripedImage struct {
	index    int
	size     opentile.Size
	tileSize opentile.Size
	grid     opentile.Size

	stripeW, stripeH int

	nativeGrid    opentile.Size
	stripeOffsets []uint64
	stripeCounts  []uint64

	// nx: output tile width / native stripe width (horizontal stripes per tile).
	// ny: output tile height / native stripe height (vertical stripes per tile).
	nx, ny int

	jpegTables []byte
	reader     io.ReaderAt

	compression opentile.Compression
	mpp         opentile.SizeMm
	pyrIndex    int
}

func newStripedImage(
	index int,
	p *tiff.Page,
	tileSize opentile.Size,
	r io.ReaderAt,
) (*stripedImage, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("ndpi: ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("ndpi: ImageLength missing")
	}
	stripeW, ok := p.TileWidth()
	if !ok {
		return nil, fmt.Errorf("ndpi: TileWidth missing (expected striped page)")
	}
	stripeH, ok := p.TileLength()
	if !ok {
		return nil, fmt.Errorf("ndpi: TileLength missing")
	}
	nativeGx, nativeGy, err := p.TileGrid()
	if err != nil {
		return nil, err
	}
	offsets, err := p.TileOffsets64()
	if err != nil {
		return nil, err
	}
	counts, err := p.TileByteCounts64()
	if err != nil {
		return nil, err
	}
	if tileSize.W%int(stripeW) != 0 || tileSize.H%int(stripeH) != 0 {
		return nil, fmt.Errorf("ndpi: adjusted tile size %v not aligned to stripe %dx%d", tileSize, stripeW, stripeH)
	}
	nx := tileSize.W / int(stripeW)
	ny := tileSize.H / int(stripeH)
	gridW := (int(iw) + tileSize.W - 1) / tileSize.W
	gridH := (int(il) + tileSize.H - 1) / tileSize.H
	tables, _ := p.JPEGTables()
	return &stripedImage{
		index:         index,
		size:          opentile.Size{W: int(iw), H: int(il)},
		tileSize:      tileSize,
		grid:          opentile.Size{W: gridW, H: gridH},
		stripeW:       int(stripeW),
		stripeH:       int(stripeH),
		nativeGrid:    opentile.Size{W: nativeGx, H: nativeGy},
		stripeOffsets: offsets,
		stripeCounts:  counts,
		nx:            nx,
		ny:            ny,
		jpegTables:    tables,
		reader:        r,
		compression:   opentile.CompressionJPEG,
	}, nil
}

func (l *stripedImage) Index() int                        { return l.index }
func (l *stripedImage) PyramidIndex() int                 { return l.pyrIndex }
func (l *stripedImage) Size() opentile.Size               { return l.size }
func (l *stripedImage) TileSize() opentile.Size           { return l.tileSize }
func (l *stripedImage) Grid() opentile.Size               { return l.grid }
func (l *stripedImage) Compression() opentile.Compression { return l.compression }
func (l *stripedImage) MPP() opentile.SizeMm              { return l.mpp }
func (l *stripedImage) FocalPlane() float64               { return 0 }

func (l *stripedImage) Tile(x, y int) ([]byte, error) {
	if x < 0 || y < 0 || x >= l.grid.W || y >= l.grid.H {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: opentile.ErrTileOutOfBounds}
	}
	fragments, err := l.readStripeFragments(x, y)
	if err != nil {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: err}
	}
	out, err := jpeg.ConcatenateScans(fragments, jpeg.ConcatOpts{
		Width:           uint16(l.tileSize.W),
		Height:          uint16(l.tileSize.H),
		JPEGTables:      l.jpegTables,
		RestartInterval: l.restartIntervalPerStripe(),
	})
	if err != nil {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: fmt.Errorf("%w: %v", opentile.ErrBadJPEGBitstream, err)}
	}
	return out, nil
}

func (l *stripedImage) TileReader(x, y int) (io.ReadCloser, error) {
	b, err := l.Tile(x, y)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (l *stripedImage) Tiles(ctx context.Context) iter.Seq2[opentile.TilePos, opentile.TileResult] {
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

// readStripeFragments reads the native-stripe JPEG fragments that compose
// the output tile at (x, y). Order: top-to-bottom, left-to-right — matching
// the scan order ConcatenateScans expects.
func (l *stripedImage) readStripeFragments(x, y int) ([][]byte, error) {
	fragments := make([][]byte, 0, l.nx*l.ny)
	for dy := 0; dy < l.ny; dy++ {
		sy := y*l.ny + dy
		for dx := 0; dx < l.nx; dx++ {
			sx := x*l.nx + dx
			if sx >= l.nativeGrid.W || sy >= l.nativeGrid.H {
				continue // edge tile may have fewer native stripes
			}
			idx := sy*l.nativeGrid.W + sx
			off := int64(l.stripeOffsets[idx])
			length := int(l.stripeCounts[idx])
			buf := make([]byte, length)
			if err := tiff.ReadAtFull(l.reader, buf, off); err != nil {
				return nil, fmt.Errorf("read stripe (%d,%d) [idx=%d]: %w", sx, sy, idx, err)
			}
			fragments = append(fragments, buf)
		}
	}
	return fragments, nil
}

// restartIntervalPerStripe computes the MCU count in one native stripe under
// NDPI's common YCbCr 4:2:0 subsampling (MCU = 16×16 pixels). Native stripes
// are typically stripeW pixels wide by 8 pixels tall; MCU row-count in the
// stripe is stripeW / 16.
func (l *stripedImage) restartIntervalPerStripe() int {
	const mcuW = 16
	if l.stripeW < mcuW {
		return 1
	}
	return l.stripeW / mcuW
}
