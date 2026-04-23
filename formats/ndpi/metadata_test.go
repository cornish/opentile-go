package ndpi

import (
	"testing"
	"time"
)

func TestParseMetadataFromFields(t *testing.T) {
	got := parseFromFields(metadataFields{
		SourceLens:              20,
		Model:                   "NanoZoomer 2.0-HT",
		DateTime:                "2014:01:07 11:22:33",
		XResolution:             [2]uint32{100000, 1},
		YResolution:             [2]uint32{100000, 1},
		ResolutionUnit:          3, // centimeters
		ZOffsetFromSlideCenter:  2500, // nm
		Reference:               "SN-1234",
	})
	if got.Magnification != 20 {
		t.Errorf("Magnification: got %v, want 20", got.Magnification)
	}
	if got.ScannerModel != "NanoZoomer 2.0-HT" {
		t.Errorf("Model: got %q", got.ScannerModel)
	}
	want := time.Date(2014, 1, 7, 11, 22, 33, 0, time.UTC)
	if !got.AcquisitionDateTime.Equal(want) {
		t.Errorf("Acq: got %v, want %v", got.AcquisitionDateTime, want)
	}
	if got.SourceLens != 20 {
		t.Errorf("SourceLens: got %v, want 20", got.SourceLens)
	}
	// FocalOffset: nm → mm (divide by 1,000,000).
	if got.FocalOffset != 2.5e-3 {
		t.Errorf("FocalOffset: got %v mm, want 0.0025", got.FocalOffset)
	}
	if got.Reference != "SN-1234" {
		t.Errorf("Reference: got %q, want SN-1234", got.Reference)
	}
	if got.ScannerManufacturer != "Hamamatsu" {
		t.Errorf("ScannerManufacturer: got %q, want Hamamatsu", got.ScannerManufacturer)
	}
}

func TestParseMetadataMissingFields(t *testing.T) {
	// All-zero fields yield zero-value metadata, no errors.
	got := parseFromFields(metadataFields{})
	if got.Magnification != 0 {
		t.Errorf("Magnification: got %v, want 0", got.Magnification)
	}
	if got.ScannerManufacturer != "Hamamatsu" {
		t.Errorf("ScannerManufacturer: got %q", got.ScannerManufacturer)
	}
	if !got.AcquisitionDateTime.IsZero() {
		t.Errorf("AcquisitionDateTime: got %v, want zero", got.AcquisitionDateTime)
	}
}
