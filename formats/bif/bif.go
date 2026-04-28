// Package bif implements opentile-go format support for Ventana BIF
// (BioImagene Image File) — a BigTIFF dialect produced by Roche's
// VENTANA DP scanner family (DP 200, DP 600) and predecessor iScan
// scanners (Coreo, HT). The format is publicly specified by Roche
// (Roche-Digital-Pathology-BIF-Whitepaper.pdf v1.0, 2020) but only
// the DP 200 generation is documented in detail; legacy iScan slides
// require openslide's permissive interpretation.
//
// Detection is a single substring match (`<iScan` in any IFD's XMP)
// shared by both spec-compliant DP slides and legacy iScan slides;
// internal classification then routes each open file to a
// spec-compliant or legacy behavioural path. See spec §4 for the
// branching rationale and `docs/deferred.md §1a` for the v0.7
// deviations from upstream Python opentile.
//
// Not affiliated with or endorsed by Roche.
package bif

import (
	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/tiff"
)

// Factory is the FormatFactory implementation for Ventana BIF.
type Factory struct{}

// New returns a BIF factory. Safe to call once and register globally.
func New() *Factory { return &Factory{} }

// Format reports the format identifier used by opentile.Tiler.Format().
func (f *Factory) Format() opentile.Format { return opentile.FormatBIF }

// Supports reports whether file looks like a BIF: BigTIFF with at
// least one IFD whose XMP packet contains the `<iScan` substring.
// See Detect for the exact rule and detection-gate verification.
func (f *Factory) Supports(file *tiff.File) bool {
	return Detect(file)
}

// Tiler is the BIF implementation of opentile.Tiler. Built up across
// v0.7 batches: T10 establishes the skeleton (factory wiring + Open
// gate); T11 adds generation classification; T12 builds the IFD
// inventory + pyramid level slice; T13 wires per-Level reads with
// serpentine remap; T14 wires the empty-tile blank-fill path; T15
// composes JPEGTables into per-tile bytes when the IFD has a shared
// header. T16+ surface associated images, metadata, and ICC profile.
type Tiler struct {
	file *tiff.File
	cfg  *opentile.Config
}

// Open constructs a BIF Tiler from a parsed TIFF file. v0.7 Batch C
// in progress: this skeleton enforces the detection gate; subsequent
// tasks (T11+) populate classification, levels, and metadata.
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	if !Detect(file) {
		return nil, opentile.ErrUnsupportedFormat
	}
	return &Tiler{file: file, cfg: cfg}, nil
}

// Format reports the BIF format identifier.
func (t *Tiler) Format() opentile.Format { return opentile.FormatBIF }

// Images returns the main pyramids carried by this file. v0.7 BIF is
// a single-image format (one pyramid per slide); populated in T12.
func (t *Tiler) Images() []opentile.Image { return nil }

// Levels is a shortcut for Images()[0].Levels(). Populated in T12.
func (t *Tiler) Levels() []opentile.Level { return nil }

// Level is a shortcut for Images()[0].Level(i). Populated in T12.
func (t *Tiler) Level(i int) (opentile.Level, error) { return nil, opentile.ErrLevelOutOfRange }

// Associated returns label / probability / thumbnail images. Populated in T16.
func (t *Tiler) Associated() []opentile.AssociatedImage { return nil }

// Metadata returns the ventana.* property mirror. Populated in T17.
func (t *Tiler) Metadata() opentile.Metadata { return opentile.Metadata{} }

// ICCProfile returns the IFD-2 ICC profile bytes (tag 34675), or nil
// if absent. Populated in T18.
func (t *Tiler) ICCProfile() []byte { return nil }

// Close releases any resources held by the Tiler. Currently a no-op:
// the underlying *tiff.File is owned by the caller.
func (t *Tiler) Close() error { return nil }
