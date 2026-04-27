package ome

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// OMEImage is one entry in the OME-XML <Image> list. Carries the
// fields opentile-go needs from each Image's <Pixels> child:
// classification anchor (Name), physical pixel size (microns/pixel),
// pixel-array dimensions, and pixel type.
//
// Fields absent in the XML stay at their zero values — the parser is
// tolerant so callers can branch on (X == 0) for "unknown".
type OMEImage struct {
	Name string

	PhysicalSizeX     float64
	PhysicalSizeY     float64
	PhysicalSizeXUnit string
	PhysicalSizeYUnit string

	SizeX int
	SizeY int

	Type string
}

// OMEMetadata is the top-level parsed view of an OME-XML document.
// Holds the Image list in document order; further interpretation
// (classification of macro / label / thumbnail vs main pyramid) is
// done in formats/ome/series.go.
type OMEMetadata struct {
	Images []OMEImage
}

// parseOMEMetadata parses an OME-XML document — the page-0
// ImageDescription of an OME TIFF file. Returns the per-Image
// inventory needed for series classification + per-level MPP.
//
// Direct port of upstream's `ome_types.from_xml(metadata)` for the
// subset of OME-XML attributes opentile-go cares about (Image Name +
// Pixels PhysicalSize / Size / Type). We deliberately ignore the
// other ~30 OME-XML elements; surfacing them is out of scope for
// v0.6 (matches upstream's narrow `Metadata()` return).
//
// Namespace-agnostic: encoding/xml struct tags don't qualify by
// namespace, so OME schemas at any version
// (2015-01, 2016-06, etc.) parse uniformly.
func parseOMEMetadata(xmlStr string) (OMEMetadata, error) {
	var doc omeDoc
	if err := xml.NewDecoder(strings.NewReader(xmlStr)).Decode(&doc); err != nil {
		return OMEMetadata{}, fmt.Errorf("ome: parse OME-XML: %w", err)
	}
	if len(doc.Images) == 0 {
		return OMEMetadata{}, fmt.Errorf("ome: OME document carries zero <Image> elements")
	}
	out := OMEMetadata{
		Images: make([]OMEImage, 0, len(doc.Images)),
	}
	for _, im := range doc.Images {
		out.Images = append(out.Images, OMEImage{
			Name:              im.Name,
			PhysicalSizeX:     im.Pixels.PhysicalSizeX,
			PhysicalSizeY:     im.Pixels.PhysicalSizeY,
			PhysicalSizeXUnit: im.Pixels.PhysicalSizeXUnit,
			PhysicalSizeYUnit: im.Pixels.PhysicalSizeYUnit,
			SizeX:             im.Pixels.SizeX,
			SizeY:             im.Pixels.SizeY,
			Type:              im.Pixels.Type,
		})
	}
	return out, nil
}

// omeDoc / omeImage / omePixels are private XML-decoding shapes. The
// public structs (OMEMetadata / OMEImage) carry the merged view
// callers consume.
type omeDoc struct {
	XMLName xml.Name   `xml:"OME"`
	Images  []omeImage `xml:"Image"`
}

type omeImage struct {
	Name   string    `xml:"Name,attr"`
	Pixels omePixels `xml:"Pixels"`
}

type omePixels struct {
	PhysicalSizeX     float64 `xml:"PhysicalSizeX,attr"`
	PhysicalSizeY     float64 `xml:"PhysicalSizeY,attr"`
	PhysicalSizeXUnit string  `xml:"PhysicalSizeXUnit,attr"`
	PhysicalSizeYUnit string  `xml:"PhysicalSizeYUnit,attr"`
	SizeX             int     `xml:"SizeX,attr"`
	SizeY             int     `xml:"SizeY,attr"`
	Type              string  `xml:"Type,attr"`
}
