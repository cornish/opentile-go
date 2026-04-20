// Package svs implements opentile-go format support for Aperio SVS files.
//
// SVS is a TIFF variant produced by Leica Aperio scanners used in digital
// pathology. This package detects SVS files, parses the Aperio metadata
// carried in the ImageDescription tag, and exposes the pyramid levels as
// opentile.Level values with raw compressed tile byte passthrough.
package svs

import (
	"fmt"
	"strings"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// aperioPrefix is the literal prefix on the ImageDescription tag of Aperio SVS
// files. Upstream opentile and openslide both key their detection off this.
const aperioPrefix = "Aperio"

// Factory is the FormatFactory implementation for SVS.
type Factory struct{}

// New returns an SVS factory. Safe to call once and register globally.
func New() *Factory { return &Factory{} }

// Format reports the format identifier used by opentile.Tiler.Format().
func (f *Factory) Format() opentile.Format { return opentile.FormatSVS }

// Supports reports whether file looks like an SVS: its first page's
// ImageDescription starts with "Aperio".
func (f *Factory) Supports(file *tiff.File) bool {
	pages := file.Pages()
	if len(pages) == 0 {
		return false
	}
	desc, ok := pages[0].ImageDescription()
	if !ok {
		return false
	}
	return strings.HasPrefix(desc, aperioPrefix)
}

// Open constructs an SVS Tiler from a parsed TIFF file.
// (Implementation completed in Task 16.)
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	return nil, fmt.Errorf("svs.Open: not yet implemented")
}
