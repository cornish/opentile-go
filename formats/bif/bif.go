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
	"bytes"
	"fmt"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/bifxml"
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

	gen        Generation         // routing decision (T11)
	iscan      *bifxml.IScan      // parsed IFD-0 metadata block; non-nil after Open
	encodeInfo *bifxml.EncodeInfo // parsed level-0 IFD's XMP; nil if absent / parse failed

	// IFD inventory (T12); built once at Open time.
	levelIFDs     []classifiedIFD // pyramid IFDs sorted by parsed level=N
	associatedIFD []classifiedIFD // label / probability / thumbnail IFDs

	// Constructed Level objects, one per levelIFDs entry. Populated
	// at Open time (T13); held in a SingleImage for Tiler.Images().
	image *opentile.SingleImage
}

// Open constructs a BIF Tiler from a parsed TIFF file. v0.7 Batch C
// in progress: this skeleton enforces the detection gate; subsequent
// tasks (T11+) populate classification, levels, and metadata.
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	if !Detect(file) {
		return nil, opentile.ErrUnsupportedFormat
	}
	iscan, err := loadIScan(file)
	if err != nil {
		return nil, err
	}
	levelIFDs, associated, _, err := inventory(file)
	if err != nil {
		return nil, err
	}
	encodeInfo := loadEncodeInfo(levelIFDs)
	levels := make([]opentile.Level, 0, len(levelIFDs))
	for i, c := range levelIFDs {
		l, err := newLevelImpl(i, c, iscan.ScanRes, encodeInfo, file.ReaderAt())
		if err != nil {
			return nil, err
		}
		levels = append(levels, l)
	}
	return &Tiler{
		file:          file,
		cfg:           cfg,
		iscan:         iscan,
		gen:           classifyGeneration(iscan),
		encodeInfo:    encodeInfo,
		levelIFDs:     levelIFDs,
		associatedIFD: associated,
		image:         opentile.NewSingleImage(levels),
	}, nil
}

// loadEncodeInfo parses the level-0 IFD's XMP into an EncodeInfo
// struct. The level-0 IFD is the first entry in levels (sorted
// ascending). Returns nil if the IFD has no XMP or parsing fails —
// non-spec-compliant slides may omit it; downstream code (TileOverlap)
// treats nil as "no overlap data".
func loadEncodeInfo(levels []classifiedIFD) *bifxml.EncodeInfo {
	if len(levels) == 0 {
		return nil
	}
	xmp, ok := levels[0].Page.XMP()
	if !ok {
		return nil
	}
	ei, err := bifxml.ParseEncodeInfo(xmp)
	if err != nil {
		return nil
	}
	return ei
}

// loadIScan locates the IFD whose XMP carries the `<iScan>` element
// and parses it. Both spec-compliant and legacy iScan slides put the
// `<iScan>` block in IFD 0's XMP, so we walk pages in order and
// parse the first match. Returns a nil *IScan only if no IFD's XMP
// contains the marker — Detect guarantees at least one does.
func loadIScan(file *tiff.File) (*bifxml.IScan, error) {
	marker := []byte(iScanMarker)
	for _, p := range file.Pages() {
		xmp, ok := p.XMP()
		if !ok {
			continue
		}
		if !bytes.Contains(xmp, marker) {
			continue
		}
		iscan, err := bifxml.ParseIScan(xmp)
		if err != nil {
			return nil, fmt.Errorf("bif: parse iScan XMP: %w", err)
		}
		return iscan, nil
	}
	return nil, fmt.Errorf("bif: no IFD carries an `%s` XMP block (Detect should have rejected)", iScanMarker)
}

// Format reports the BIF format identifier.
func (t *Tiler) Format() opentile.Format { return opentile.FormatBIF }

// Images returns the main pyramids carried by this file. BIF is a
// single-image format — always one Image regardless of AOI count.
func (t *Tiler) Images() []opentile.Image {
	return []opentile.Image{t.image}
}

// Levels is a shortcut for Images()[0].Levels().
func (t *Tiler) Levels() []opentile.Level { return t.image.Levels() }

// Level is a shortcut for Images()[0].Level(i).
func (t *Tiler) Level(i int) (opentile.Level, error) { return t.image.Level(i) }

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
