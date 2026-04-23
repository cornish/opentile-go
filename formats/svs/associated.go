package svs

import (
	"fmt"
	"io"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/jpeg"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// classifyAssociatedKind returns the AssociatedImage kind for an SVS page or
// empty string if the page is not an associated image. Mirrors
// tifffile._series_svs: page 1 is always the thumbnail (positional); any
// non-tiled page at index >= 2 with SubFileType 9 is the macro (overview),
// SubFileType 1 is the label. Everything else is either a pyramid level
// (handled separately) or ignored.
//
// Do not consult ImageDescription for this decision; upstream does not, and
// the description strings are inconsistent across Aperio versions.
func classifyAssociatedKind(pageIdx int, subfileType uint32, tiled bool) string {
	if pageIdx == 1 {
		return "thumbnail"
	}
	if tiled {
		return ""
	}
	switch subfileType {
	case 9:
		return "overview"
	case 1:
		return "label"
	}
	return ""
}

// newAssociatedImage dispatches construction by kind. Thumbnail and overview
// are striped JPEG assembled via ConcatenateScans; label is raw strip
// passthrough (codec as advertised by the TIFF Compression tag).
func newAssociatedImage(kind string, p *tiff.Page, r io.ReaderAt) (opentile.AssociatedImage, error) {
	switch kind {
	case "thumbnail", "overview":
		return newStripedJPEGAssociated(kind, p, r)
	case "label":
		return newStripedLabel(p, r)
	}
	return nil, fmt.Errorf("svs: unknown associated kind %q", kind)
}

// stripedJPEGAssociated is the SVS AssociatedImage implementation for
// thumbnail and overview pages. Bytes() assembles a standalone JPEG from the
// TIFF strips via internal/jpeg.ConcatenateScans, injecting the page's
// JPEGTables and an APP14 "Adobe" marker to signal RGB colorspace (Aperio
// stores non-standard RGB JPEG).
type stripedJPEGAssociated struct {
	kind            string
	size            opentile.Size
	stripOffsets    []uint64
	stripCounts     []uint64
	jpegTables      []byte
	restartInterval int
	reader          io.ReaderAt
}

func (a *stripedJPEGAssociated) Kind() string                      { return a.kind }
func (a *stripedJPEGAssociated) Size() opentile.Size               { return a.size }
func (a *stripedJPEGAssociated) Compression() opentile.Compression { return opentile.CompressionJPEG }

func (a *stripedJPEGAssociated) Bytes() ([]byte, error) {
	fragments := make([][]byte, len(a.stripOffsets))
	for i := range a.stripOffsets {
		buf := make([]byte, a.stripCounts[i])
		if err := tiff.ReadAtFull(a.reader, buf, int64(a.stripOffsets[i])); err != nil {
			return nil, fmt.Errorf("svs: read associated strip %d: %w", i, err)
		}
		fragments[i] = buf
	}
	if a.size.W > 0xFFFF || a.size.H > 0xFFFF {
		return nil, fmt.Errorf("svs: associated %s %dx%d exceeds SOF uint16", a.kind, a.size.W, a.size.H)
	}
	return jpeg.ConcatenateScans(fragments, jpeg.ConcatOpts{
		Width:           uint16(a.size.W),
		Height:          uint16(a.size.H),
		JPEGTables:      a.jpegTables,
		ColorspaceFix:   true,
		RestartInterval: a.restartInterval,
	})
}

func newStripedJPEGAssociated(kind string, p *tiff.Page, r io.ReaderAt) (*stripedJPEGAssociated, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("svs: associated %s ImageWidth missing", kind)
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("svs: associated %s ImageLength missing", kind)
	}
	offsets, err := p.ScalarArrayU64(tiff.TagStripOffsets)
	if err != nil {
		return nil, fmt.Errorf("svs: associated %s strip offsets: %w", kind, err)
	}
	counts, err := p.ScalarArrayU64(tiff.TagStripByteCounts)
	if err != nil {
		return nil, fmt.Errorf("svs: associated %s strip counts: %w", kind, err)
	}
	if len(offsets) != len(counts) {
		return nil, fmt.Errorf("svs: associated %s strip tag mismatch: offsets=%d counts=%d", kind, len(offsets), len(counts))
	}
	if len(offsets) == 0 {
		return nil, fmt.Errorf("svs: associated %s has no strips", kind)
	}
	tables, _ := p.JPEGTables()

	// RestartInterval: upstream writes one DRI giving MCUs-per-strip so the
	// decoder knows each RSTn boundary is MCUs-per-strip apart. RowsPerStrip
	// (tag 278) is the strip height; MCU size is 16x16 for YCbCr 4:2:0 (the
	// Aperio default). If RowsPerStrip is missing, fall back to 0 (no DRI,
	// verbatim concat — acceptable only for single-strip pages).
	restartInterval := 0
	if len(offsets) > 1 {
		rps, ok := p.ScalarU32(tiff.TagRowsPerStrip)
		if !ok || rps == 0 {
			return nil, fmt.Errorf("svs: associated %s multi-strip but missing RowsPerStrip", kind)
		}
		// MCUs per strip = (padded_width / 16) * (rows_per_strip / 16).
		// Round up both dimensions to an MCU boundary to stay safe.
		const mcu = 16
		mcusX := (int(iw) + mcu - 1) / mcu
		mcusY := (int(rps) + mcu - 1) / mcu
		restartInterval = mcusX * mcusY
	}

	return &stripedJPEGAssociated{
		kind:            kind,
		size:            opentile.Size{W: int(iw), H: int(il)},
		stripOffsets:    offsets,
		stripCounts:     counts,
		jpegTables:      tables,
		restartInterval: restartInterval,
		reader:          r,
	}, nil
}

// stripedLabel is the SVS AssociatedImage implementation for the label page.
// Label compression varies (LZW=5 in all three CMU fixtures, but can be JPEG
// or uncompressed); upstream SvsLabelImage returns the raw first strip bytes
// and advertises whatever Compression the TIFF carries. Callers are expected
// to decode with an external codec if needed.
type stripedLabel struct {
	size         opentile.Size
	compression  opentile.Compression
	stripOffsets []uint64
	stripCounts  []uint64
	reader       io.ReaderAt
}

func (a *stripedLabel) Kind() string                      { return "label" }
func (a *stripedLabel) Size() opentile.Size               { return a.size }
func (a *stripedLabel) Compression() opentile.Compression { return a.compression }

// Bytes returns strip 0's raw compressed bytes.
//
// This matches the upstream Python opentile SvsLabelImage.get_tile((0,0))
// behavior, which returns a single strip regardless of how many strips the
// TIFF carries. For multi-strip LZW labels (typical for CMU fixtures) this
// means the returned blob is a valid-but-truncated LZW stream representing
// only the top RowsPerStrip rows of the label, not the full image.
//
// Proper multi-strip stitching is deferred to v0.3 (see docs/deferred.md).
func (a *stripedLabel) Bytes() ([]byte, error) {
	if len(a.stripOffsets) == 0 || len(a.stripCounts) == 0 {
		return nil, fmt.Errorf("svs: label has no strips")
	}
	buf := make([]byte, a.stripCounts[0])
	if err := tiff.ReadAtFull(a.reader, buf, int64(a.stripOffsets[0])); err != nil {
		return nil, fmt.Errorf("svs: read label strip 0: %w", err)
	}
	return buf, nil
}

func newStripedLabel(p *tiff.Page, r io.ReaderAt) (*stripedLabel, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return nil, fmt.Errorf("svs: label ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return nil, fmt.Errorf("svs: label ImageLength missing")
	}
	offsets, err := p.ScalarArrayU64(tiff.TagStripOffsets)
	if err != nil {
		return nil, fmt.Errorf("svs: label strip offsets: %w", err)
	}
	counts, err := p.ScalarArrayU64(tiff.TagStripByteCounts)
	if err != nil {
		return nil, fmt.Errorf("svs: label strip counts: %w", err)
	}
	comp, _ := p.Compression()
	return &stripedLabel{
		size:         opentile.Size{W: int(iw), H: int(il)},
		compression:  tiffCompressionToOpentile(comp),
		stripOffsets: offsets,
		stripCounts:  counts,
		reader:       r,
	}, nil
}

// tiffCompressionToOpentile maps TIFF tag 259 numeric values to the
// opentile.Compression enum. Unknown codes (including vendor-specific ones)
// become CompressionUnknown so consumers can still get the raw bytes but
// know the codec is not advertised.
//
// Shared by both the tiled level path (tiled.go) and the associated-image
// path (this file) — kept in one place so adding a new code (e.g. LZW) is a
// single-line change covering every SVS consumer.
func tiffCompressionToOpentile(tiffCode uint32) opentile.Compression {
	switch tiffCode {
	case 1:
		return opentile.CompressionNone
	case 5:
		return opentile.CompressionLZW
	case 7:
		return opentile.CompressionJPEG
	case 33003, 33005: // APERIO_JP2000_YCBC / APERIO_JP2000_RGB
		return opentile.CompressionJP2K
	}
	return opentile.CompressionUnknown
}
