package ndpi

import (
	"fmt"
	"io"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/jpegturbo"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// overviewImage is an NDPI "Macro" page exposed as an AssociatedImage with
// Kind() == "overview". Its Bytes() passes through the raw JPEG payload
// without modification (no cgo required).
type overviewImage struct {
	size        opentile.Size
	compression opentile.Compression
	offset      uint64
	length      uint64
	reader      io.ReaderAt
}

func newOverviewImage(p *tiff.Page, r io.ReaderAt) (*overviewImage, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("ndpi: overview ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("ndpi: overview ImageLength missing")
	}
	// NDPI Macro pages use StripOffsets (273) / StripByteCounts (279) rather
	// than TileOffsets (324) / TileByteCounts (325).
	offsets, err := p.ScalarArrayU64(tiff.TagStripOffsets)
	if err != nil {
		return nil, fmt.Errorf("ndpi: overview offsets: %w", err)
	}
	counts, err := p.ScalarArrayU64(tiff.TagStripByteCounts)
	if err != nil {
		return nil, fmt.Errorf("ndpi: overview counts: %w", err)
	}
	if len(offsets) != 1 || len(counts) != 1 {
		return nil, fmt.Errorf("ndpi: overview expected 1 strip, got offsets=%d counts=%d", len(offsets), len(counts))
	}
	return &overviewImage{
		size:        opentile.Size{W: int(iw), H: int(il)},
		compression: opentile.CompressionJPEG,
		offset:      offsets[0],
		length:      counts[0],
		reader:      r,
	}, nil
}

func (o *overviewImage) Kind() string                      { return "overview" }
func (o *overviewImage) Size() opentile.Size               { return o.size }
func (o *overviewImage) Compression() opentile.Compression { return o.compression }

func (o *overviewImage) Bytes() ([]byte, error) {
	buf := make([]byte, o.length)
	if err := tiff.ReadAtFull(o.reader, buf, int64(o.offset)); err != nil {
		return nil, fmt.Errorf("ndpi: read overview: %w", err)
	}
	return buf, nil
}

// labelImage is the cropped left portion of the macro image, exposed with
// Kind() == "label". Upstream default crop is 0.0 → 0.3 of macro width
// (caller-configurable at construction). Requires cgo for the crop.
type labelImage struct {
	overview *overviewImage
	cropFrom int // left pixel offset in source (MCU-aligned)
	cropTo   int // right pixel offset in source (exclusive, MCU-aligned)
	cropH    int // MCU-aligned height
}

// newLabelImage returns a labelImage whose Bytes() crops the overview to
// [0, crop * overview.Width) horizontally, snapped down to the nearest MCU
// boundary. mcuW / mcuH are the MCU dimensions of the overview's JPEG —
// derive them via jpeg.MCUSizeOf on the overview bytes (16x16 for YCbCr
// 4:2:0; 8x8 for the 4:4:4 case Hamamatsu actually uses on macro pages).
//
// Width is MCU-rounded the way Python opentile's _calculate_crop is
// (int(W * crop / mcuW) * mcuW). Height is also rounded down to the nearest
// mcuH multiple here because internal/jpegturbo.Crop uses TJXOPT_PERFECT
// and rejects non-MCU-aligned regions; Python's PyTurboJPEG.crop_multiple
// tolerates ragged crops, which is why upstream passes the un-rounded page
// height. A v0.4 fix could route non-aligned cases through
// CropWithBackground for full Python parity.
func newLabelImage(overview *overviewImage, crop float64, mcuW, mcuH int) *labelImage {
	pixelTo := int(float64(overview.size.W) * crop)
	pixelTo = (pixelTo / mcuW) * mcuW
	if pixelTo <= 0 {
		pixelTo = mcuW
	}
	return &labelImage{
		overview: overview,
		cropFrom: 0,
		cropTo:   pixelTo,
		cropH:    (overview.size.H / mcuH) * mcuH,
	}
}

func (l *labelImage) Kind() string                      { return "label" }
func (l *labelImage) Size() opentile.Size               { return opentile.Size{W: l.cropTo - l.cropFrom, H: l.cropH} }
func (l *labelImage) Compression() opentile.Compression { return l.overview.compression }

func (l *labelImage) Bytes() ([]byte, error) {
	src, err := l.overview.Bytes()
	if err != nil {
		return nil, err
	}
	return jpegturbo.Crop(src, jpegturbo.Region{
		X: l.cropFrom, Y: 0, Width: l.cropTo - l.cropFrom, Height: l.cropH,
	})
}
