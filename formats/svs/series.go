package svs

import "errors"

// errNoPages is returned when classifyPages is called on an empty page list.
var errNoPages = errors.New("svs: file has no pages")

// errBaseNotTiled is returned when page 0 is not tiled. Aperio (and tifffile)
// require the baseline to be tiled; a non-tiled page 0 is not an SVS pyramid.
var errBaseNotTiled = errors.New("svs: base page (0) must be tiled")

// pageMeta is the minimal SVS-relevant metadata for a single TIFF page,
// extracted once per Open() call so the classifier can be a pure function over
// a small struct rather than the heavyweight *tiff.Page.
type pageMeta struct {
	Tiled       bool
	Reduced     bool   // NewSubfileType bit 0
	SubfileType uint32 // raw NewSubfileType value (used to pick Macro vs Label)
}

// classification names which page indices map to which SVS series. Absent
// associated images carry index -1.
type classification struct {
	Levels    []int // page indices forming the Baseline pyramid, in order
	Thumbnail int   // -1 if absent
	Label     int   // -1 if absent
	Macro     int   // -1 if absent
}

// classifyPages applies tifffile's _series_svs algorithm
// (tifffile/tifffile.py:5218) to assign each TIFF page to an SVS series.
//
// Algorithm:
//  1. Page 0 must be tiled; it is the first Baseline level.
//  2. With only one page, return Baseline-only (no associated images).
//  3. Page 1 is the Thumbnail iff non-tiled. A tiled page 1 means the file
//     omitted the thumbnail — treat it as the next Baseline level.
//  4. Walk pages 2..N appending tiled-and-non-reduced pages to Baseline; stop
//     at the first non-tiled or reduced page.
//  5. Up to 2 trailing pages are Label/Macro by SubfileType: 9 → Macro, else
//     → Label. Order is preserved (Macro can come before Label or vice-versa).
//     Anything beyond the second trailing page is ignored.
//
// The function is pure over its input metadata so it can be tested without
// constructing real TIFF files.
func classifyPages(metas []pageMeta) (classification, error) {
	if len(metas) == 0 {
		return classification{}, errNoPages
	}
	if !metas[0].Tiled {
		return classification{}, errBaseNotTiled
	}

	c := classification{
		Levels:    []int{0},
		Thumbnail: -1,
		Label:     -1,
		Macro:     -1,
	}

	if len(metas) == 1 {
		return c, nil
	}

	// Page 1: thumbnail when non-tiled, otherwise a Baseline level.
	idx := 1
	if !metas[1].Tiled {
		c.Thumbnail = 1
		idx = 2
	}

	// Walk remaining pages as Baseline while tiled and not reduced.
	for idx < len(metas) {
		m := metas[idx]
		if !m.Tiled || m.Reduced {
			break
		}
		c.Levels = append(c.Levels, idx)
		idx++
	}

	// Up to 2 trailing pages → Label / Macro by SubfileType.
	for i := 0; i < 2 && idx < len(metas); i++ {
		if metas[idx].SubfileType == 9 {
			c.Macro = idx
		} else {
			c.Label = idx
		}
		idx++
	}

	return c, nil
}
