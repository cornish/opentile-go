package ndpi

import (
	"time"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tiff"
)

// Metadata is NDPI-specific slide metadata. Embeds opentile.Metadata for the
// cross-format fields (Magnification, scanner info, AcquisitionDateTime).
type Metadata struct {
	opentile.Metadata
	SourceLens  float64 // objective magnification from Hamamatsu SourceLens tag (65420)
	FocalDepth  float64 // mm, from FocalDepth tag (if present)
	FocalOffset float64 // mm, from ZOffsetFromSlideCenter tag (nanometers → mm)
	Reference   string  // scanner reference/serial
}

// NDPI vendor-private tag IDs.
const (
	tagSourceLens             uint16 = 65420
	tagZOffsetFromSlideCenter uint16 = 65427
	tagFocalDepth             uint16 = 65432
	tagReference              uint16 = 65442
)

// Standard TIFF tag IDs used by the NDPI metadata parser.
const (
	tagModel    uint16 = 272
	tagDateTime uint16 = 306
)

// metadataFields is the un-marshaled shape consumed by parseFromFields.
// Production paths populate it from *tiff.Page via parseMetadata; tests
// construct it directly.
type metadataFields struct {
	SourceLens             uint32
	Model                  string
	DateTime               string
	XResolution            [2]uint32
	YResolution            [2]uint32
	ResolutionUnit         uint32
	ZOffsetFromSlideCenter uint32
	FocalDepth             uint32
	Reference              string
}

// parseMetadata reads NDPI metadata from the first TIFF page.
func parseMetadata(p *tiff.Page) (Metadata, error) {
	var f metadataFields
	if v, ok := p.ScalarU32(tagSourceLens); ok {
		f.SourceLens = v
	}
	f.Model, _ = p.ASCII(tagModel)
	f.DateTime, _ = p.ASCII(tagDateTime)
	if numer, denom, ok := p.XResolution(); ok {
		f.XResolution = [2]uint32{numer, denom}
	}
	if numer, denom, ok := p.YResolution(); ok {
		f.YResolution = [2]uint32{numer, denom}
	}
	if v, ok := p.ResolutionUnit(); ok {
		f.ResolutionUnit = v
	}
	if v, ok := p.ScalarU32(tagZOffsetFromSlideCenter); ok {
		f.ZOffsetFromSlideCenter = v
	}
	if v, ok := p.ScalarU32(tagFocalDepth); ok {
		f.FocalDepth = v
	}
	f.Reference, _ = p.ASCII(tagReference)
	return parseFromFields(f), nil
}

// parseFromFields builds a Metadata from its un-marshaled tag values. Kept
// separate from parseMetadata so unit tests can construct metadata without
// needing a *tiff.Page.
func parseFromFields(f metadataFields) Metadata {
	md := Metadata{
		SourceLens:  float64(f.SourceLens),
		FocalOffset: float64(f.ZOffsetFromSlideCenter) / 1_000_000.0,
		FocalDepth:  float64(f.FocalDepth) / 1_000_000.0,
		Reference:   f.Reference,
	}
	md.Magnification = float64(f.SourceLens)
	md.ScannerManufacturer = "Hamamatsu"
	md.ScannerModel = f.Model
	if f.Model != "" {
		md.ScannerSoftware = []string{f.Model}
	}
	if t, err := time.Parse("2006:01:02 15:04:05", f.DateTime); err == nil {
		md.AcquisitionDateTime = t
	}
	return md
}

// MetadataOf returns the NDPI-specific metadata if t is an NDPI Tiler.
// Walks Tiler wrappers (mirrors svs.MetadataOf) to accommodate the
// fileCloser wrapper that opentile.OpenFile returns.
func MetadataOf(t opentile.Tiler) (*Metadata, bool) {
	const maxHops = 16
	for i := 0; t != nil && i <= maxHops; i++ {
		if nt, ok := t.(*tiler); ok {
			return &nt.md, true
		}
		u, ok := t.(interface{ UnwrapTiler() opentile.Tiler })
		if !ok {
			return nil, false
		}
		t = u.UnwrapTiler()
	}
	return nil, false
}

// tiler is the NDPI implementation of opentile.Tiler; fully defined in
// Task 20. Declared here so MetadataOf compiles.
type tiler struct {
	md         Metadata
	levels     []opentile.Level
	associated []opentile.AssociatedImage
	icc        []byte
}

// Interface satisfaction stubs for Task 20 implementation.
func (t *tiler) Format() opentile.Format                { return opentile.FormatNDPI }
func (t *tiler) Levels() []opentile.Level               { return nil }
func (t *tiler) Level(i int) (opentile.Level, error)    { return nil, nil }
func (t *tiler) Associated() []opentile.AssociatedImage { return nil }
func (t *tiler) Metadata() opentile.Metadata            { return t.md.Metadata }
func (t *tiler) ICCProfile() []byte                     { return t.icc }
func (t *tiler) Close() error                           { return nil }
