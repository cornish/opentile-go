// Package ndpi implements opentile-go format support for Hamamatsu NDPI
// files. NDPI is a TIFF variant with vendor-private tags (FileFormat,
// Magnification, ZOffsetFromSlideCenter, etc.) and pyramid levels stored as
// horizontal stripes — typically 8 pixels tall — that must be reshaped into
// square output tiles at the JPEG marker level.
//
// This package detects NDPI files via the FileFormat (65420) vendor tag AND
// the Make (271) tag, ports tifffile's _series_ndpi page classification via
// tag 65421 (Magnification, FLOAT), and exposes pyramid levels as
// opentile.Level values. Striped levels use pure-Go marker concatenation
// (internal/jpeg); one-frame levels and the label image require cgo
// (internal/jpegturbo).
package ndpi

import (
	"fmt"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// tagMake is the standard TIFF Make tag (camera/scanner manufacturer).
const tagMake uint16 = 271

// Factory is the FormatFactory implementation for NDPI.
type Factory struct{}

// New returns an NDPI factory. Safe to register globally.
func New() *Factory { return &Factory{} }

// Format reports the format identifier used by opentile.Tiler.Format().
func (f *Factory) Format() opentile.Format { return opentile.FormatNDPI }

// Supports reports whether file looks like an NDPI. Per tifffile line 10608:
// NDPI requires BOTH FileFormat (65420) AND Make (271).
func (f *Factory) Supports(file *tiff.File) bool {
	pages := file.Pages()
	if len(pages) == 0 {
		return false
	}
	p := pages[0]
	if _, ok := p.ScalarU32(tagFileFormat); !ok {
		return false
	}
	if _, ok := p.ASCII(tagMake); !ok {
		return false
	}
	return true
}

// Open constructs an NDPI Tiler from a parsed TIFF file.
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	pages := file.Pages()
	if len(pages) == 0 {
		return nil, fmt.Errorf("ndpi: file has no pages")
	}
	md, err := parseMetadata(pages[0])
	if err != nil {
		return nil, err
	}

	// Resolve the requested tile size and snap to stripe width.
	reqSize := opentile.Size{W: 512, H: 512}
	if sz, set := cfg.TileSize(); set {
		if sz.W != sz.H {
			return nil, fmt.Errorf("ndpi: tile size must be square, got %v", sz)
		}
		reqSize = sz
	}
	smallestStripe := smallestStripeWidth(pages)
	adjusted := AdjustTileSize(reqSize.W, smallestStripe)

	var levels []opentile.Level
	var associated []opentile.AssociatedImage
	var overview *overviewImage
	levelIdx := 0
	for _, p := range pages {
		kind := classifyPage(p)
		switch kind {
		case pageLevel:
			// Distinguish striped from one-frame: tiled pages have TileWidth.
			if _, tiled := p.TileWidth(); tiled {
				lvl, err := newStripedImage(levelIdx, p, adjusted, file.ReaderAt())
				if err != nil {
					return nil, fmt.Errorf("ndpi: level %d: %w", levelIdx, err)
				}
				levels = append(levels, lvl)
			} else {
				lvl, err := newOneFrameImage(levelIdx, p, adjusted, file.ReaderAt())
				if err != nil {
					return nil, fmt.Errorf("ndpi: level %d: %w", levelIdx, err)
				}
				levels = append(levels, lvl)
			}
			levelIdx++
		case pageMacro:
			ov, err := newOverviewImage(p, file.ReaderAt())
			if err != nil {
				return nil, fmt.Errorf("ndpi: overview: %w", err)
			}
			overview = ov
			associated = append(associated, ov)
		case pageMap:
			// v0.2: Map pages are skipped. v0.3+ may expose them as associated.
		case pageUnknown:
			// Skip pages with no magnification tag; they're malformed or not
			// part of the standard NDPI layout.
		}
	}
	if overview != nil {
		// Default label crop: 0 → 30% of macro width. MCU sizes default to
		// 16x16 (YCbCr 4:2:0 — Hamamatsu standard).
		associated = append(associated, newLabelImage(overview, 0.3, 16, 16))
	}
	return &tiler{md: md, levels: levels, associated: associated}, nil
}

// smallestStripeWidth walks all tiled pages and returns the smallest
// TileWidth found, or 0 if no pages are tiled.
func smallestStripeWidth(pages []*tiff.Page) int {
	smallest := 0
	for _, p := range pages {
		tw, ok := p.TileWidth()
		if !ok {
			continue
		}
		if smallest == 0 || int(tw) < smallest {
			smallest = int(tw)
		}
	}
	return smallest
}
