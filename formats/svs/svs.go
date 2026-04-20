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
func (f *Factory) Open(file *tiff.File, cfg *opentile.Config) (opentile.Tiler, error) {
	pages := file.Pages()
	if len(pages) == 0 {
		return nil, fmt.Errorf("svs: file has no pages")
	}
	basePage := pages[0]
	desc, ok := basePage.ImageDescription()
	if !ok {
		return nil, fmt.Errorf("svs: base page missing ImageDescription")
	}
	md, err := parseDescription(desc)
	if err != nil {
		return nil, err
	}

	// Pyramid levels are tiled; non-tiled pages (thumbnail, label, macro)
	// are the substrate for associated-image support (v0.3) and are skipped
	// here. Level indices are contiguous (0..N-1) in pyramid order and do
	// not correspond to physical page indices in the TIFF.
	levels := make([]opentile.Level, 0, len(pages))
	baseSize, err := pageSize(basePage)
	if err != nil {
		return nil, err
	}
	levelIdx := 0
	for pageIdx, p := range pages {
		if _, ok := p.TileWidth(); !ok {
			continue // non-tiled page; defer to v0.3 associated-image support
		}
		lvl, err := newTiledImage(levelIdx, p, baseSize, md.MPP, file.ReaderAt(), cfg)
		if err != nil {
			return nil, fmt.Errorf("svs: page %d (level %d): %w", pageIdx, levelIdx, err)
		}
		levels = append(levels, lvl)
		levelIdx++
	}
	icc, _ := basePage.ICCProfile()
	return &tiler{md: md, levels: levels, icc: icc}, nil
}

// pageSize returns the (ImageWidth, ImageLength) as opentile.Size.
func pageSize(p *tiff.Page) (opentile.Size, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return opentile.Size{}, fmt.Errorf("ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return opentile.Size{}, fmt.Errorf("ImageLength missing")
	}
	return opentile.Size{W: int(iw), H: int(il)}, nil
}

// tiler is the SVS implementation of opentile.Tiler.
type tiler struct {
	md     Metadata
	levels []opentile.Level
	icc    []byte
}

func (t *tiler) Format() opentile.Format                { return opentile.FormatSVS }
func (t *tiler) Levels() []opentile.Level               { return t.levels }
func (t *tiler) Associated() []opentile.AssociatedImage { return nil }
func (t *tiler) Metadata() opentile.Metadata            { return t.md.Metadata }
func (t *tiler) ICCProfile() []byte                     { return t.icc }
func (t *tiler) Close() error                           { return nil }
func (t *tiler) Level(i int) (opentile.Level, error) {
	if i < 0 || i >= len(t.levels) {
		return nil, opentile.ErrLevelOutOfRange
	}
	return t.levels[i], nil
}

// MetadataOf returns the SVS-specific metadata if t is an SVS Tiler, otherwise
// (nil, false). Use this to read the Aperio extras (MPP, SoftwareLine,
// Filename) that are not visible through the common opentile.Metadata struct.
//
//	if md, ok := svs.MetadataOf(tiler); ok {
//	    fmt.Println(md.MPP, md.SoftwareLine)
//	}
func MetadataOf(t opentile.Tiler) (*Metadata, bool) {
	svsT, ok := t.(*tiler)
	if !ok {
		return nil, false
	}
	// Return a pointer into the tiler's stored metadata. t.md is populated
	// once at Open time and never mutated; the returned pointer is safe to
	// hold for the lifetime of the Tiler.
	return &svsT.md, true
}
