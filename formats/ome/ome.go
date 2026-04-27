// Package ome implements opentile-go format support for OME TIFF
// files — a TIFF dialect carrying OME-XML metadata in the first
// page's ImageDescription, with reduced-resolution pyramid levels
// stored as TIFF SubIFDs of the base page.
//
// Direct port of Python opentile 0.20.0's formats/ome/ subtree
// (Apache 2.0, Sectra AB), with one deliberate deviation: multi-image
// OME files (where several main pyramids share a single TIFF
// container — Leica-2.ome.tiff is one) expose every pyramid via the
// new opentile.Tiler.Images() API. Upstream's base Tiler loop
// silently overwrites _level_series_index on each match and surfaces
// only the last main pyramid; we treat that as an upstream oversight
// rather than intentional behaviour.
package ome

import (
	"errors"
	"fmt"
	"strings"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/tiff"
)

// omeDescriptionSuffix is the substring `is_ome` looks for at the end
// of the first page's ImageDescription. tifffile.py:10125-10129:
//
//	if self.index != 0 or not self.description:
//	    return False
//	return self.description[-10:].strip().endswith('OME>')
const omeDescriptionSuffix = "OME>"

// Factory is the FormatFactory implementation for OME TIFF.
type Factory struct{}

// New returns an OME factory. Safe to call once and register globally.
func New() *Factory { return &Factory{} }

// Format reports the format identifier used by opentile.Tiler.Format().
func (f *Factory) Format() opentile.Format { return opentile.FormatOME }

// Supports reports whether file looks like an OME TIFF: its first
// page's ImageDescription's last 10 characters, after stripping
// trailing whitespace, end with `OME>` (i.e. the closing tag of the
// `<OME>` root element). Direct port of tifffile's `is_ome`
// predicate.
func (f *Factory) Supports(file *tiff.File) bool {
	pages := file.Pages()
	if len(pages) == 0 {
		return false
	}
	desc, ok := pages[0].ImageDescription()
	if !ok || desc == "" {
		return false
	}
	tail := desc
	if len(tail) > 10 {
		tail = tail[len(tail)-10:]
	}
	return strings.HasSuffix(strings.TrimSpace(tail), omeDescriptionSuffix)
}

// Open constructs an OME Tiler from a parsed TIFF file. v0.6
// in-progress: Image / Level / AssociatedImage plumbing lands in
// follow-on tasks. Returns a sentinel error so the detection path
// can be exercised end-to-end without surfacing a half-built tiler.
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	return nil, errOMENotYetImplemented
}

// errOMENotYetImplemented is returned by Open until the format
// package's Image / Level / Associated plumbing lands. Removed before
// v0.6 ships.
var errOMENotYetImplemented = errors.New("ome: Open not yet implemented (v0.6 in progress)")

// unused stub keeps the package importable when later tasks haven't
// yet introduced their helpers.
var _ = fmt.Sprintf
