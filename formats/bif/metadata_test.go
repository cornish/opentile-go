package bif

import (
	"bytes"
	"testing"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/tiff"
)

// TestMetadataPopulatesIScanFields: Open + Metadata() returns the
// common fields populated from <iScan>; MetadataOf returns BIF-only
// fields (Generation, ScanRes, AOIs, ...) on the same tiler.
func TestMetadataPopulatesIScanFields(t *testing.T) {
	xmp := []byte(`<iScan ScannerModel="VENTANA DP 200" Magnification="40" ScanRes="0.25" UnitNumber="2000515" BuildVersion="1.1.0.15854" ScanWhitePoint="235" Z-layers="1"><AOI0 Left="297" Top="2323" Right="574" Bottom="2069"/></iScan>`)
	data := buildBIFLikeBigTIFF(t, []iFDSpec{
		{xmp: xmp, description: "Label_Image"},
		{description: "level=0 mag=40 quality=95", imageWidth: 64, imageLength: 64, tileWidth: 64, tileLength: 64},
	})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	tiler, err := New().Open(f, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	common := tiler.Metadata()
	if common.ScannerModel != "VENTANA DP 200" {
		t.Errorf("ScannerModel: got %q, want %q", common.ScannerModel, "VENTANA DP 200")
	}
	if common.ScannerManufacturer != "Roche" {
		t.Errorf("ScannerManufacturer: got %q, want %q", common.ScannerManufacturer, "Roche")
	}
	if common.Magnification != 40 {
		t.Errorf("Magnification: got %v, want 40", common.Magnification)
	}
	if common.ScannerSerial != "2000515" {
		t.Errorf("ScannerSerial: got %q, want %q", common.ScannerSerial, "2000515")
	}
	if len(common.ScannerSoftware) != 1 || common.ScannerSoftware[0] != "1.1.0.15854" {
		t.Errorf("ScannerSoftware: got %v, want [1.1.0.15854]", common.ScannerSoftware)
	}

	bm, ok := MetadataOf(tiler)
	if !ok {
		t.Fatal("MetadataOf: ok=false on a real BIF Tiler")
	}
	if bm.Generation != "spec-compliant" {
		t.Errorf("Generation: got %q, want %q", bm.Generation, "spec-compliant")
	}
	if bm.ScanRes != 0.25 {
		t.Errorf("ScanRes: got %v, want 0.25", bm.ScanRes)
	}
	if !bm.ScanWhitePointPresent {
		t.Error("ScanWhitePointPresent: false, want true")
	}
	if bm.ScanWhitePoint != 235 {
		t.Errorf("ScanWhitePoint: got %d, want 235", bm.ScanWhitePoint)
	}
	if bm.ZLayers != 1 {
		t.Errorf("ZLayers: got %d, want 1", bm.ZLayers)
	}
	if bm.ImageDescription != "level=0 mag=40 quality=95" {
		t.Errorf("ImageDescription: got %q", bm.ImageDescription)
	}
	if len(bm.AOIs) != 1 {
		t.Errorf("AOIs: got %d, want 1", len(bm.AOIs))
	}
}

// TestMetadataLegacyIScanDefaults: a slide without ScannerModel
// reports manufacturer "Roche" + a sensible model fallback, and
// the BIF Generation is "legacy-iscan".
func TestMetadataLegacyIScanDefaults(t *testing.T) {
	xmp := []byte(`<iScan Magnification="40" ScanRes="0.2325" UnitNumber="BI10N0306" BuildVersion="3.3.1.1"/>`)
	data := buildBIFLikeBigTIFF(t, []iFDSpec{
		{xmp: xmp, description: "Label Image"},
		{description: "level=0 mag=40 quality=90", imageWidth: 64, imageLength: 64, tileWidth: 64, tileLength: 64},
	})
	f, _ := tiff.Open(bytes.NewReader(data), int64(len(data)))
	tiler, err := New().Open(f, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	common := tiler.Metadata()
	if common.ScannerModel != "VENTANA iScan" {
		t.Errorf("ScannerModel: got %q, want fallback %q", common.ScannerModel, "VENTANA iScan")
	}
	if common.ScannerManufacturer != "Roche" {
		t.Errorf("ScannerManufacturer: got %q, want %q", common.ScannerManufacturer, "Roche")
	}
	bm, _ := MetadataOf(tiler)
	if bm.Generation != "legacy-iscan" {
		t.Errorf("Generation: got %q, want %q", bm.Generation, "legacy-iscan")
	}
	if bm.ScanWhitePointPresent {
		t.Error("ScanWhitePointPresent: true, want false (legacy fixture has no attribute)")
	}
}

// TestMetadataOfRejectsNonBIFTiler: MetadataOf returns (nil, false)
// for any non-BIF Tiler (mirrors svs.MetadataOf).
func TestMetadataOfRejectsNonBIFTiler(t *testing.T) {
	if md, ok := MetadataOf(nonBIFTiler{}); md != nil || ok {
		t.Errorf("MetadataOf(non-BIF): got (%v, %v), want (nil, false)", md, ok)
	}
}

// nonBIFTiler is a stub satisfying opentile.Tiler so MetadataOf has
// a non-*Tiler input to reject.
type nonBIFTiler struct{}

func (nonBIFTiler) Format() opentile.Format               { return opentile.FormatSVS }
func (nonBIFTiler) Images() []opentile.Image              { return nil }
func (nonBIFTiler) Levels() []opentile.Level              { return nil }
func (nonBIFTiler) Level(int) (opentile.Level, error)     { return nil, opentile.ErrLevelOutOfRange }
func (nonBIFTiler) Associated() []opentile.AssociatedImage { return nil }
func (nonBIFTiler) Metadata() opentile.Metadata           { return opentile.Metadata{} }
func (nonBIFTiler) ICCProfile() []byte                    { return nil }
func (nonBIFTiler) Close() error                          { return nil }

// TestMetadataIsCachedNotRecomputed: two consecutive Metadata calls
// return equal common-field structs; MetadataOf returns the same
// pointer.
func TestMetadataIsCached(t *testing.T) {
	xmp := []byte(`<iScan ScannerModel="VENTANA DP 200"/>`)
	data := buildBIFLikeBigTIFF(t, []iFDSpec{
		{xmp: xmp, description: "Label_Image"},
		{description: "level=0 mag=40 quality=95", imageWidth: 64, imageLength: 64, tileWidth: 64, tileLength: 64},
	})
	f, _ := tiff.Open(bytes.NewReader(data), int64(len(data)))
	tiler, _ := New().Open(f, nil)
	a, _ := MetadataOf(tiler)
	b, _ := MetadataOf(tiler)
	if a != b {
		t.Error("MetadataOf returned different pointers; the second call should hit the cache")
	}
}
