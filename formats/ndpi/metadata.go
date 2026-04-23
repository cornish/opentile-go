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
	SourceLens  float64 // objective magnification from tag 65421 (equivalent to Magnification)
	FocalOffset float64 // mm, from ZOffsetFromSlideCenter tag 65424 (nanometers → mm)
	Reference   string  // scanner serial, tag 65442
}

// NDPI vendor-private tag IDs (verified against cgohlke/tifffile NDPI_TAGS registry).
const (
	tagFileFormat             uint16 = 65420 // present iff NDPI; version marker
	tagMagnification          uint16 = 65421 // FLOAT; -1 = Macro, -2 = Map, >0 = source lens
	tagXOffsetFromSlideCenter uint16 = 65422 // SLONG
	tagYOffsetFromSlideCenter uint16 = 65423 // SLONG
	tagZOffsetFromSlideCenter uint16 = 65424 // SLONG (nanometers; focal plane)
	tagTissueIndex            uint16 = 65425
	tagSlideLabel             uint16 = 65427 // ASCII
	tagCaptureMode            uint16 = 65441
	tagReference              uint16 = 65442 // ScannerSerialNumber (ASCII)
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
	Magnification          float32 // from tag 65421; may be negative for Macro/Map
	Model                  string
	DateTime               string
	XResolution            [2]uint32
	YResolution            [2]uint32
	ResolutionUnit         uint32
	ZOffsetFromSlideCenter int32 // SLONG, nanometers (note: signed)
	Reference              string
}

// parseMetadata reads NDPI metadata from the first TIFF page.
func parseMetadata(p *tiff.Page) (Metadata, error) {
	var f metadataFields
	if v, ok := p.Float32(tagMagnification); ok {
		f.Magnification = v
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
	// ZOffsetFromSlideCenter is SLONG (signed). Use the raw uint32 value
	// reinterpreted as int32.
	if v, ok := p.ScalarU32(tagZOffsetFromSlideCenter); ok {
		f.ZOffsetFromSlideCenter = int32(v)
	}
	f.Reference, _ = p.ASCII(tagReference)
	return parseFromFields(f), nil
}

// parseFromFields builds a Metadata from its un-marshaled tag values. Kept
// separate from parseMetadata so unit tests can construct metadata without
// needing a *tiff.Page.
func parseFromFields(f metadataFields) Metadata {
	md := Metadata{
		FocalOffset: float64(f.ZOffsetFromSlideCenter) / 1_000_000.0, // nm → mm
		Reference:   f.Reference,
	}
	// NDPI magnification may be negative for associated-image pages (-1 Macro,
	// -2 Map). For the pyramid-level metadata path we clamp to >=0, so a
	// negative value means "not a real magnification" and Magnification stays 0.
	if f.Magnification > 0 {
		md.Magnification = float64(f.Magnification)
		md.SourceLens = float64(f.Magnification)
	}
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
