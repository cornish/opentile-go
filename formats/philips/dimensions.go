// Package philips reads tiles from Philips IntelliSite Pathology Solution
// TIFF whole-slide images. It is a direct port of Python opentile's
// formats/philips/ subtree (Apache 2.0, Sectra AB).
package philips

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// dicomXML is the minimal shape we need to extract DICOM_PIXEL_SPACING
// (and later other DICOM_* attributes) from Philips's ImageDescription
// XML. The schema is a flat list of <Attribute Name="..."> elements
// inside a single <DataObject> root; we don't need a typed model of
// every possible Philips attribute.
type dicomXML struct {
	Attributes []dicomAttr `xml:"Attribute"`
}

type dicomAttr struct {
	Name string `xml:"Name,attr"`
	Text string `xml:",chardata"`
}

// parsePixelSpacingPairs walks the XML and returns every
// DICOM_PIXEL_SPACING entry's (w, h) ratio, in document order. Each
// entry's text is a whitespace-separated pair of quoted floats — e.g.
// `"0.000247746" "0.000247746"`. Quotes are stripped before parsing.
func parsePixelSpacingPairs(xmlStr string) ([][2]float64, error) {
	var d dicomXML
	if err := xml.Unmarshal([]byte(xmlStr), &d); err != nil {
		return nil, fmt.Errorf("philips: parse metadata XML: %w", err)
	}
	var out [][2]float64
	for _, a := range d.Attributes {
		if a.Name != "DICOM_PIXEL_SPACING" {
			continue
		}
		w, h, ok := parseTwoFloats(a.Text)
		if !ok {
			return nil, fmt.Errorf("philips: malformed DICOM_PIXEL_SPACING value %q", a.Text)
		}
		out = append(out, [2]float64{w, h})
	}
	return out, nil
}

// parseTwoFloats strips literal quote characters from s and splits on
// whitespace, returning the first two parsed floats. Mirrors
// upstream's `text.replace('"', '').split()`.
func parseTwoFloats(s string) (float64, float64, bool) {
	s = strings.ReplaceAll(s, `"`, "")
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return 0, 0, false
	}
	w, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, false
	}
	h, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, false
	}
	return w, h, true
}

// computeCorrectedSizes parses Philips DICOM-XML metadata and returns
// the corrected (W, H) for each tiled pyramid page in document order.
//
// Direct port of tifffile._philips_load_pages (tifffile.py:6477-6540).
// The first DICOM_PIXEL_SPACING entry calibrates the baseline mm scale
// from baseW/baseH (the on-disk page-0 dimensions); each subsequent
// entry produces a corrected size for the next tiled page via:
//
//	mmW = baseW * w_base
//	mmH = baseH * h_base
//	corrected[k].W = ceil(mmW / w_{k+1})
//	corrected[k].H = ceil(mmH / h_{k+1})
//
// So N PS entries produce N-1 corrected sizes; the first tiled page
// (the "baseline" pyramid level) gets corrected[0], the second tiled
// page gets corrected[1], etc.
//
// Returns an error if no DICOM_PIXEL_SPACING entries are present (a
// malformed Philips slide; upstream asserts).
func computeCorrectedSizes(xmlStr string, baseW, baseH int) ([][2]int, error) {
	pairs, err := parsePixelSpacingPairs(xmlStr)
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("philips: no DICOM_PIXEL_SPACING entries in metadata")
	}

	wBase, hBase := pairs[0][0], pairs[0][1]
	mmW := float64(baseW) * wBase
	mmH := float64(baseH) * hBase

	out := make([][2]int, 0, len(pairs)-1)
	for _, p := range pairs[1:] {
		w := int(math.Ceil(mmW / p[0]))
		h := int(math.Ceil(mmH / p[1]))
		out = append(out, [2]int{w, h})
	}
	return out, nil
}
