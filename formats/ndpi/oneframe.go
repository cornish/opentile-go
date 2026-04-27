package ndpi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"iter"
	"sync"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/jpeg"
	"github.com/cornish/opentile-go/internal/jpegturbo"
	"github.com/cornish/opentile-go/internal/tiff"
)

// oneFrameImage is an NDPI Level backed by a single JPEG per page (typical
// for lower pyramid levels that fit in one JPEG). Output tiles are produced
// by lossless MCU-aligned crop via libjpeg-turbo (internal/jpegturbo).
type oneFrameImage struct {
	index       int
	size        opentile.Size
	tileSize    opentile.Size
	grid        opentile.Size
	compression opentile.Compression
	mpp         opentile.SizeMm
	pyrIndex    int

	// paddedJPEG is the full-level JPEG payload with its SOF rewritten to
	// MCU-aligned dimensions. Built lazily on first tile read and cached
	// for the lifetime of the level. Single-entry guarantee comes from
	// paddedJPEGOnce; the previous v0.2 implementation used a plain bool
	// guard which only worked because the sole caller (extendedOnce.Do
	// in getExtendedFrame) provided the memory barrier transitively.
	// sync.Once makes the contract explicit so a future caller adding a
	// non-extendedOnce.Do entry can't regress concurrency safety.
	paddedJPEGOnce sync.Once
	paddedJPEG     []byte
	paddedJPEGErr  error
	mcuW, mcuH     int

	// extendedFrame is the padded JPEG further widened to a tile-aligned
	// size (ceil(size/tileSize)*tileSize), with the OOB region filled via
	// the DCT callback. Built lazily on first tile read and cached. Every
	// tile crop uses this as its source, so per-tile hot path is a plain
	// MCU-aligned Crop — no CUSTOMFILTER overhead after the first call.
	//
	// Mirrors opentile.NdpiOneFrameImage._read_extended_frame (see
	// ndpi_image.py:375-405).
	extendedOnce sync.Once
	extendedErr  error
	extendedJPEG []byte
	extendedSize opentile.Size

	reader io.ReaderAt
	page   *tiff.Page
}

func newOneFrameImage(
	index int,
	p *tiff.Page,
	tileSize opentile.Size,
	r io.ReaderAt,
) (*oneFrameImage, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("ndpi: ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("ndpi: ImageLength missing")
	}
	gridW := (int(iw) + tileSize.W - 1) / tileSize.W
	gridH := (int(il) + tileSize.H - 1) / tileSize.H
	return &oneFrameImage{
		index:       index,
		size:        opentile.Size{W: int(iw), H: int(il)},
		tileSize:    tileSize,
		grid:        opentile.Size{W: gridW, H: gridH},
		compression: opentile.CompressionJPEG,
		reader:      r,
		page:        p,
	}, nil
}

func (l *oneFrameImage) Index() int                        { return l.index }
func (l *oneFrameImage) PyramidIndex() int                 { return l.pyrIndex }
func (l *oneFrameImage) Size() opentile.Size               { return l.size }
func (l *oneFrameImage) TileSize() opentile.Size           { return l.tileSize }
func (l *oneFrameImage) Grid() opentile.Size               { return l.grid }
func (l *oneFrameImage) Compression() opentile.Compression { return l.compression }
func (l *oneFrameImage) MPP() opentile.SizeMm              { return l.mpp }
func (l *oneFrameImage) FocalPlane() float64               { return 0 }

func (l *oneFrameImage) Tile(x, y int) ([]byte, error) {
	if x < 0 || y < 0 || x >= l.grid.W || y >= l.grid.H {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: opentile.ErrTileOutOfBounds}
	}
	frame, err := l.getExtendedFrame()
	if err != nil {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: err}
	}
	region := jpegturbo.Region{X: x * l.tileSize.W, Y: y * l.tileSize.H, Width: l.tileSize.W, Height: l.tileSize.H}
	out, err := jpegturbo.Crop(frame, region)
	if err != nil {
		return nil, &opentile.TileError{Level: l.index, X: x, Y: y, Err: err}
	}
	return out, nil
}

func (l *oneFrameImage) TileReader(x, y int) (io.ReadCloser, error) {
	b, err := l.Tile(x, y)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (l *oneFrameImage) Tiles(ctx context.Context) iter.Seq2[opentile.TilePos, opentile.TileResult] {
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

// getPaddedJPEG reads the level's JPEG payload once and returns a slice
// where the SOF dimensions are rounded up to MCU boundaries (safe for
// tjTransform's TJXOPT_PERFECT). Cached for the level's lifetime via
// paddedJPEGOnce.
func (l *oneFrameImage) getPaddedJPEG() ([]byte, error) {
	l.paddedJPEGOnce.Do(func() {
		l.paddedJPEG, l.paddedJPEGErr = l.buildPaddedJPEG()
	})
	return l.paddedJPEG, l.paddedJPEGErr
}

// buildPaddedJPEG carries the actual work of getPaddedJPEG; separated
// out so paddedJPEGOnce.Do receives a no-arg closure with no return
// values.
func (l *oneFrameImage) buildPaddedJPEG() ([]byte, error) {
	// NDPI one-frame level pages use StripOffsets (273) / StripByteCounts (279)
	// rather than TileOffsets (324) / TileByteCounts (325).
	offsets, err := l.page.ScalarArrayU64(tiff.TagStripOffsets)
	if err != nil {
		return nil, fmt.Errorf("one-frame page missing StripOffsets: %w", err)
	}
	counts, err := l.page.ScalarArrayU64(tiff.TagStripByteCounts)
	if err != nil {
		return nil, fmt.Errorf("one-frame page missing StripByteCounts: %w", err)
	}
	if len(offsets) != 1 || len(counts) != 1 {
		return nil, fmt.Errorf("one-frame page expected 1 offset/count, got %d/%d", len(offsets), len(counts))
	}
	buf := make([]byte, counts[0])
	if err := tiff.ReadAtFull(l.reader, buf, int64(offsets[0])); err != nil {
		return nil, fmt.Errorf("read one-frame JPEG: %w", err)
	}
	// Determine MCU size from SOF inside buf.
	var sof *jpeg.SOF
	for seg, err := range jpeg.Scan(bytes.NewReader(buf)) {
		if err != nil {
			return nil, fmt.Errorf("%w: %w", opentile.ErrBadJPEGBitstream, err)
		}
		if seg.Marker == jpeg.SOF0 {
			sof, err = jpeg.ParseSOF(seg.Payload)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", opentile.ErrBadJPEGBitstream, err)
			}
			break
		}
	}
	if sof == nil {
		return nil, fmt.Errorf("%w: SOF not found in one-frame page", opentile.ErrBadJPEGBitstream)
	}
	mcuW, mcuH := sof.MCUSize()
	l.mcuW, l.mcuH = mcuW, mcuH
	paddedW := roundUp(l.size.W, mcuW)
	paddedH := roundUp(l.size.H, mcuH)
	if paddedW > 0xFFFF || paddedH > 0xFFFF {
		return nil, fmt.Errorf("%w: one-frame level %dx%d exceeds SOF uint16 range", opentile.ErrBadJPEGBitstream, paddedW, paddedH)
	}
	if paddedW == l.size.W && paddedH == l.size.H {
		return buf, nil
	}
	rewrote, err := jpeg.ReplaceSOFDimensions(buf, uint16(paddedW), uint16(paddedH))
	if err != nil {
		return nil, fmt.Errorf("pad SOF: %w", err)
	}
	return rewrote, nil
}

// getExtendedFrame produces the tile-aligned "extended frame" from which
// every output tile is cropped in-bounds. Mirrors upstream opentile's
// NdpiOneFrameImage._read_extended_frame:
//
//  1. Pad the raw level JPEG's SOF up to MCU boundaries (via
//     getPaddedJPEG) so libjpeg-turbo accepts it as a well-formed image.
//  2. Widen the frame again to a tile-aligned size via CropWithBackground
//     with X=0, Y=0 and width/height = ceil(size/tile)*tile. libjpeg-turbo
//     treats this as a "crop extension" (crop_width > output_width allowed
//     when crop_xoffset==0); the CUSTOMFILTER callback fills the newly
//     added blocks with a background color.
//
// After this, every Tile(x, y) crop lies wholly inside extendedFrame and
// uses plain Crop — the OOB fill happens once per level, not per tile.
func (l *oneFrameImage) getExtendedFrame() ([]byte, error) {
	l.extendedOnce.Do(func() {
		padded, err := l.getPaddedJPEG()
		if err != nil {
			l.extendedErr = err
			return
		}
		extW := roundUp(l.size.W, l.tileSize.W)
		extH := roundUp(l.size.H, l.tileSize.H)
		if extW > 0xFFFF || extH > 0xFFFF {
			l.extendedErr = fmt.Errorf("%w: extended frame %dx%d exceeds SOF uint16 range",
				opentile.ErrBadJPEGBitstream, extW, extH)
			return
		}
		// Fast path: padded dims are already tile-aligned; the extended
		// frame equals the padded JPEG.
		paddedW := roundUp(l.size.W, l.mcuW)
		paddedH := roundUp(l.size.H, l.mcuH)
		if paddedW == extW && paddedH == extH {
			l.extendedJPEG = padded
			l.extendedSize = opentile.Size{W: extW, H: extH}
			return
		}
		frame, err := jpegturbo.CropWithBackground(padded, jpegturbo.Region{
			X: 0, Y: 0, Width: extW, Height: extH,
		})
		if err != nil {
			l.extendedErr = fmt.Errorf("extend frame: %w", err)
			return
		}
		l.extendedJPEG = frame
		l.extendedSize = opentile.Size{W: extW, H: extH}
	})
	if l.extendedErr != nil {
		return nil, l.extendedErr
	}
	return l.extendedJPEG, nil
}

func roundUp(n, to int) int {
	if n%to == 0 {
		return n
	}
	return n + (to - n%to)
}
