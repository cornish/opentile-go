package svs

import (
	"errors"
	"reflect"
	"testing"
)

// classifyPages cases mirror tifffile._series_svs (tifffile.py:5218). The
// classifier itself is a pure function over per-page metadata so we can drive
// it from synthetic inputs without building real TIFF bytes.

func TestClassifyPagesEmpty(t *testing.T) {
	_, err := classifyPages(nil)
	if !errors.Is(err, errNoPages) {
		t.Fatalf("classifyPages(nil): want errNoPages, got %v", err)
	}
}

func TestClassifyPagesBaseNotTiled(t *testing.T) {
	metas := []pageMeta{
		{Tiled: false}, // page 0 must be tiled for SVS
	}
	_, err := classifyPages(metas)
	if !errors.Is(err, errBaseNotTiled) {
		t.Fatalf("classifyPages: want errBaseNotTiled, got %v", err)
	}
}

func TestClassifyPagesSingleBaseline(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true},
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0},
		Thumbnail: -1,
		Label:     -1,
		Macro:     -1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// CMU-1-Small-Region.svs ground truth from tifffile:
//
//	page 0: tiled=True  reduced=False subfile=0
//	page 1: tiled=False reduced=False subfile=0   (Thumbnail)
//	page 2: tiled=False reduced=True  subfile=1   (Label)
//	page 3: tiled=False reduced=True  subfile=9   (Macro)
func TestClassifyPagesCMU1SmallRegion(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: true, SubfileType: 1},
		{Tiled: false, Reduced: true, SubfileType: 9},
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0},
		Thumbnail: 1,
		Label:     2,
		Macro:     3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// CMU-1.svs ground truth from tifffile:
//
//	page 0: tiled=True  reduced=False subfile=0   (Baseline lvl 0)
//	page 1: tiled=False reduced=False subfile=0   (Thumbnail)
//	page 2: tiled=True  reduced=False subfile=0   (Baseline lvl 1)
//	page 3: tiled=True  reduced=False subfile=0   (Baseline lvl 2)
//	page 4: tiled=False reduced=True  subfile=1   (Label)
//	page 5: tiled=False reduced=True  subfile=9   (Macro)
func TestClassifyPagesCMU1(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0},
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: true, SubfileType: 1},
		{Tiled: false, Reduced: true, SubfileType: 9},
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0, 2, 3},
		Thumbnail: 1,
		Label:     4,
		Macro:     5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// svs_40x_bigtiff.svs ground truth from tifffile (no Label):
//
//	page 0: tiled=True  reduced=False subfile=0
//	page 1: tiled=False reduced=False subfile=0
//	page 2: tiled=True  reduced=False subfile=0
//	page 3: tiled=True  reduced=False subfile=0
//	page 4: tiled=True  reduced=False subfile=0
//	page 5: tiled=False reduced=True  subfile=9
func TestClassifyPagesNoLabel(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0},
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: true, SubfileType: 9},
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0, 2, 3, 4},
		Thumbnail: 1,
		Label:     -1,
		Macro:     5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// Hypothetical: SVS with Label but no Macro. Faithfully exercises tifffile's
// "up to 2 trailing pages, classify each by subfileType" rule.
func TestClassifyPagesNoMacro(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0}, // thumbnail
		{Tiled: false, Reduced: true, SubfileType: 1},  // label
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0},
		Thumbnail: 1,
		Label:     2,
		Macro:     -1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// Hypothetical: baseline pyramid with no thumbnail, no label, no macro.
// tifffile assumes page 1 is the thumbnail when len(pages) >= 2; we follow
// that contract — page 1 is the thumbnail iff it is non-tiled. A tiled page 1
// is treated as a Baseline level (the file simply omits the thumbnail).
func TestClassifyPagesNoThumbnailTiledP1(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: true, Reduced: false, SubfileType: 0}, // would-be thumbnail, but tiled
		{Tiled: true, Reduced: false, SubfileType: 0},
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0, 1, 2},
		Thumbnail: -1,
		Label:     -1,
		Macro:     -1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// Reduced bit on a tiled page terminates the Baseline walk. tifffile uses this
// rule (`if not page.is_tiled or page.is_reduced: break`) to find where the
// pyramid ends and Label/Macro begins. Defends against a slide whose Label
// happens to also carry the tile tags.
func TestClassifyPagesReducedTerminatesBaseline(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0}, // thumbnail
		{Tiled: true, Reduced: false, SubfileType: 0},  // baseline lvl 1
		{Tiled: true, Reduced: true, SubfileType: 1},   // tiled+reduced ⇒ stop walking
		{Tiled: false, Reduced: true, SubfileType: 9},  // macro
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0, 2},
		Thumbnail: 1,
		Label:     3, // first trailing page, subfileType != 9
		Macro:     4,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// tifffile classifies trailing pages purely by subfileType (==9 → Macro,
// else → Label) and processes at most 2 of them. If the file presents Macro
// before Label, we should preserve that ordering.
func TestClassifyPagesMacroBeforeLabel(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0}, // thumbnail
		{Tiled: false, Reduced: true, SubfileType: 9},  // macro
		{Tiled: false, Reduced: true, SubfileType: 1},  // label
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0},
		Thumbnail: 1,
		Label:     3,
		Macro:     2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}

// tifffile caps trailing-page processing at 2 pages. Anything beyond that is
// silently ignored (no fifth-or-later associated image is emitted).
func TestClassifyPagesIgnoresExtrasBeyondTwo(t *testing.T) {
	metas := []pageMeta{
		{Tiled: true, Reduced: false, SubfileType: 0},
		{Tiled: false, Reduced: false, SubfileType: 0}, // thumbnail
		{Tiled: false, Reduced: true, SubfileType: 1},  // label
		{Tiled: false, Reduced: true, SubfileType: 9},  // macro
		{Tiled: false, Reduced: true, SubfileType: 9},  // 3rd trailing — ignored
	}
	got, err := classifyPages(metas)
	if err != nil {
		t.Fatalf("classifyPages: %v", err)
	}
	want := classification{
		Levels:    []int{0},
		Thumbnail: 1,
		Label:     2,
		Macro:     3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classifyPages = %+v, want %+v", got, want)
	}
}
