// Package ndpi implements opentile-go format support for Hamamatsu NDPI
// files. NDPI is a TIFF variant with vendor-private tags (SourceLens,
// ZOffsetFromSlideCenter, etc.) and pyramid levels stored as horizontal
// stripes — typically 8 pixels tall — that must be reshaped into square
// output tiles at the JPEG marker level.
//
// This package detects NDPI files via the SourceLens (65420) vendor tag,
// parses NDPI-specific metadata, and exposes pyramid levels as opentile.Level
// values. Striped levels use pure-Go marker concatenation (internal/jpeg);
// one-frame levels and the label image require cgo (internal/jpegturbo).
package ndpi

import (
	"fmt"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// ndpiSourceLensTag is the Hamamatsu vendor-private tag used for NDPI
// detection and objective-magnification extraction.
const ndpiSourceLensTag uint16 = 65420

// Factory is the FormatFactory implementation for NDPI.
type Factory struct{}

// New returns an NDPI factory. Safe to register globally.
func New() *Factory { return &Factory{} }

// Format reports the format identifier used by opentile.Tiler.Format().
func (f *Factory) Format() opentile.Format { return opentile.FormatNDPI }

// Supports reports whether file looks like an NDPI by checking the first
// page for the SourceLens (65420) vendor-private tag.
func (f *Factory) Supports(file *tiff.File) bool {
	pages := file.Pages()
	if len(pages) == 0 {
		return false
	}
	_, ok := pages[0].ScalarU32(ndpiSourceLensTag)
	return ok
}

// Open is replaced in Task 20 with the real NDPI opener.
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	return nil, fmt.Errorf("ndpi.Open: not yet implemented")
}
