package philips

import (
	"errors"
	"strings"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// philipsSoftwarePrefix is the literal prefix on the Software tag (305)
// that identifies a Philips IntelliSite Pathology Solution scan.
// Upstream tifffile keys detection off this:
//
//	software[:10] == 'Philips DP' AND description[-16:].strip().endswith('</DataObject>')
const philipsSoftwarePrefix = "Philips DP"

// philipsDescriptionSuffix is the closing tag of the DICOM-XML blob
// Philips writes into the ImageDescription tag (270). Upstream pins
// detection on the suffix to avoid false positives from generic
// </DataObject>-bearing XML in non-Philips TIFFs.
const philipsDescriptionSuffix = "</DataObject>"

// Factory is the FormatFactory implementation for Philips TIFF.
type Factory struct{}

// New returns a Philips factory. Safe to call once and register globally.
func New() *Factory { return &Factory{} }

// Format reports the format identifier used by opentile.Tiler.Format().
func (f *Factory) Format() opentile.Format { return opentile.FormatPhilips }

// Supports reports whether file looks like a Philips TIFF: its first
// page's Software tag starts with "Philips DP" AND its ImageDescription
// tag ends in </DataObject> (after stripping trailing whitespace).
//
// Direct port of tifffile's `is_philips` predicate
// (tifffile.py:10267-10271). Both signals are required — either alone
// has too high a false-positive rate.
func (f *Factory) Supports(file *tiff.File) bool {
	pages := file.Pages()
	if len(pages) == 0 {
		return false
	}
	sw, ok := pages[0].Software()
	if !ok || !strings.HasPrefix(sw, philipsSoftwarePrefix) {
		return false
	}
	desc, ok := pages[0].ImageDescription()
	if !ok {
		return false
	}
	return strings.HasSuffix(strings.TrimSpace(desc), philipsDescriptionSuffix)
}

// Open constructs a Philips Tiler from a parsed TIFF file. v0.5
// in-progress: the level / associated-image plumbing is added in
// follow-on tasks. For now Open returns a sentinel error so the
// detection path can be exercised end-to-end without surfacing a
// half-built tiler.
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	return nil, errPhilipsNotYetImplemented
}

// errPhilipsNotYetImplemented is returned by Open until the format
// package's level / associated / metadata plumbing lands. Removed before
// v0.5 ships.
var errPhilipsNotYetImplemented = errors.New("philips: Open not yet implemented (v0.5 in progress)")
