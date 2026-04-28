package bif

import (
	"strings"
	"time"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/bifxml"
	"github.com/cornish/opentile-go/internal/tiff"
)

// Metadata is the BIF-specific slide metadata. It embeds
// opentile.Metadata so the common fields (Magnification, scanner
// identity, AcquisitionDateTime) are populated via the embedded
// struct; BIF-specific fields (Generation, ScanRes, ScanWhitePoint,
// AOIs, ImageDescription) live on the outer struct.
//
// Consumers read the common fields via opentile.Tiler.Metadata() as
// usual; to read the BIF-specific fields, pass the Tiler to
// bif.MetadataOf.
type Metadata struct {
	opentile.Metadata

	// Generation is the routing decision: "spec-compliant" for
	// VENTANA DP scanners (200, 600, future); "legacy-iscan" for
	// pre-DP iScan slides and any unrecognised iScan ScannerModel.
	Generation string

	// ScanRes is the base-level microns/pixel from <iScan>/@ScanRes.
	// Same value applies to X and Y per spec — BIF doesn't carry
	// anisotropic pixels.
	ScanRes float64

	// ScanWhitePoint is the white-fill luminance for empty tiles
	// (`<iScan>/@ScanWhitePoint`). Only populated when
	// ScanWhitePointPresent is true; otherwise the consumer should
	// default to 255 (matches T9's blankTile fallback).
	ScanWhitePoint        uint8
	ScanWhitePointPresent bool

	// ZLayers is `<iScan>/@Z-layers`. v0.7 only surfaces in-focus
	// tiles even when ZLayers > 1; the value is exposed so consumers
	// can detect volumetric slides and look elsewhere if needed.
	ZLayers int

	// ImageDescription mirrors the level-0 IFD's ImageDescription
	// tag verbatim (e.g., "level=0 mag=40 quality=95"). Useful for
	// debugging or for surfacing the per-level JPEG quality.
	ImageDescription string

	// AOIs is the list of areas-of-interest declared in the
	// <iScan> XMP (one entry per AOI<N> sub-element). For
	// single-AOI slides (both our local fixtures), this has one
	// entry. The bounding rectangles are in the slide's physical
	// coordinate system (origin at lower-left, Y up).
	AOIs []bifxml.AOI

	// AOIOrigins is the list of AOI origins from the level-0 IFD's
	// <EncodeInfo>/<AoiOrigin> elements (one per AOI). Origins are
	// in image-space pixel coordinates (top-left origin), always
	// multiples of the tile size per spec. Empty for legacy iScan
	// slides that don't carry EncodeInfo, or when EncodeInfo failed
	// to parse.
	AOIOrigins []bifxml.AoiOrigin

	// EncodeInfoVer is the level-0 EncodeInfo @Ver attribute (must
	// be ≥ 2 per spec; the parser exposes the raw value here).
	EncodeInfoVer int
}

// tilerUnwrapper matches the unexported wrapper interface returned
// by opentile.OpenFile. Mirrors svs.tilerUnwrapper.
type tilerUnwrapper interface {
	UnwrapTiler() opentile.Tiler
}

// maxTilerUnwrapHops caps the number of UnwrapTiler calls MetadataOf
// will make. The realistic chain length is 1 (just the file-closer
// wrapper); 16 is ample headroom while still preventing infinite
// loops on a wrapper that cycles.
const maxTilerUnwrapHops = 16

// MetadataOf returns the BIF-specific metadata if t is a BIF Tiler,
// otherwise (nil, false). Walks any number of wrappers (e.g., the
// file-closer wrapper returned by opentile.OpenFile) before
// asserting on the concrete type. Mirrors svs.MetadataOf.
//
//	if md, ok := bif.MetadataOf(tiler); ok {
//	    use md.Generation, md.ScanRes, ...
//	}
func MetadataOf(t opentile.Tiler) (*Metadata, bool) {
	for i := 0; t != nil && i <= maxTilerUnwrapHops; i++ {
		if bt, ok := t.(*Tiler); ok {
			return bt.metadata(), true
		}
		u, ok := t.(tilerUnwrapper)
		if !ok {
			return nil, false
		}
		t = u.UnwrapTiler()
	}
	return nil, false
}

// metadata builds the Metadata struct from this Tiler's parsed
// IScan + EncodeInfo. Called once at Open time and cached on the
// Tiler.
func (t *Tiler) metadata() *Metadata {
	if t.cachedMetadata != nil {
		return t.cachedMetadata
	}
	md := &Metadata{
		Generation: t.gen.String(),
	}
	if t.iscan != nil {
		md.Magnification = t.iscan.Magnification
		md.ScanRes = t.iscan.ScanRes
		md.ScanWhitePoint = t.iscan.ScanWhitePoint
		md.ScanWhitePointPresent = t.iscan.ScanWhitePointPresent
		md.ZLayers = t.iscan.ZLayers
		md.AOIs = append([]bifxml.AOI(nil), t.iscan.AOIs...)

		// ScannerManufacturer: every iScan-tagged slide is from
		// Roche / VENTANA Tissue Diagnostics regardless of model.
		md.ScannerManufacturer = "Roche"
		md.ScannerModel = t.iscan.ScannerModel
		if md.ScannerModel == "" {
			md.ScannerModel = "VENTANA iScan" // best-effort label for legacy slides
		}
		if t.iscan.BuildVersion != "" {
			md.ScannerSoftware = []string{t.iscan.BuildVersion}
		}
		md.ScannerSerial = t.iscan.UnitNumber

		// AcquisitionDateTime: the iScan node's BuildDate is the
		// scanner-software build, NOT the scan-time. Real
		// scan-time lives in the IFD's TIFF tag DateTime
		// (yyyy:mm:dd HH:MM:SS) — populated below.
	}

	// Pull TIFF tag DateTime / Software / ImageDescription from the
	// level-0 IFD when available.
	if len(t.levelIFDs) > 0 {
		p := t.levelIFDs[0].Page
		if v, ok := p.ImageDescription(); ok {
			md.ImageDescription = strings.TrimSpace(v)
		}
		if v, ok := p.ASCII(tiff.TagDateTime); ok {
			if ts, err := time.Parse("2006:01:02 15:04:05", strings.TrimSpace(v)); err == nil {
				md.AcquisitionDateTime = ts
			}
		}
	}

	if t.encodeInfo != nil {
		md.EncodeInfoVer = t.encodeInfo.Ver
		md.AOIOrigins = append([]bifxml.AoiOrigin(nil), t.encodeInfo.AoiOrigins...)
	}

	t.cachedMetadata = md
	return md
}
