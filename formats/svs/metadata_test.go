package svs

import (
	"testing"
	"time"
)

func TestParseDescription(t *testing.T) {
	desc := "Aperio Image Library v11.2.1\n" +
		"46000x32914 [0,100 46000x32714] (240x240) JPEG/RGB Q=30|" +
		"AppMag = 20|MPP = 0.4990|Date = 02/02/2017|Time = 11:22:33|" +
		"ScanScope ID = SS1234|Filename = CMU-1"

	md, err := parseDescription(desc)
	if err != nil {
		t.Fatalf("parseDescription: %v", err)
	}
	if md.Magnification != 20 {
		t.Errorf("Magnification: got %v, want 20", md.Magnification)
	}
	if md.MPP != 0.499 {
		t.Errorf("MPP: got %v, want 0.499", md.MPP)
	}
	if md.ScannerSerial != "SS1234" {
		t.Errorf("ScannerSerial: got %q, want SS1234", md.ScannerSerial)
	}
	if md.SoftwareLine != "Aperio Image Library v11.2.1" {
		t.Errorf("SoftwareLine: got %q", md.SoftwareLine)
	}
	want := time.Date(2017, 2, 2, 11, 22, 33, 0, time.UTC)
	if !md.AcquisitionDateTime.Equal(want) {
		t.Errorf("AcquisitionDateTime: got %v, want %v", md.AcquisitionDateTime, want)
	}
}

func TestParseDescriptionMissingFields(t *testing.T) {
	md, err := parseDescription("Aperio Image Library v11.2.1\n256x256 (16x16) JPEG/RGB")
	if err != nil {
		t.Fatalf("parseDescription: %v", err)
	}
	if md.Magnification != 0 || md.MPP != 0 || md.ScannerSerial != "" {
		t.Errorf("expected zero values for missing fields, got %+v", md)
	}
}

func TestParseDescriptionRejectsNonAperio(t *testing.T) {
	_, err := parseDescription("Hamamatsu Ndpi\n...")
	if err == nil {
		t.Fatal("expected error on non-Aperio description")
	}
}

func TestParseDescriptionRejectsGarbageAppMag(t *testing.T) {
	desc := "Aperio Image Library v1.0\n1x1 (1x1)|AppMag = notanumber"
	_, err := parseDescription(desc)
	if err == nil {
		t.Fatal("expected error on garbage AppMag")
	}
}

func TestParseDescriptionRejectsGarbageMPP(t *testing.T) {
	desc := "Aperio Image Library v1.0\n1x1 (1x1)|MPP = not_a_float"
	_, err := parseDescription(desc)
	if err == nil {
		t.Fatal("expected error on garbage MPP")
	}
}

func TestParseDescriptionTwoDigitYear(t *testing.T) {
	// Real-world Aperio slides (CMU-1-Small-Region.svs, CMU-1.svs, JP2K-33003-1.svs)
	// use two-digit years in Date field.
	desc := "Aperio Image Library v11.2.1\n" +
		"256x256 (16x16) JPEG/RGB|" +
		"Date = 12/29/09|Time = 09:59:15"
	md, err := parseDescription(desc)
	if err != nil {
		t.Fatalf("parseDescription: %v", err)
	}
	want := time.Date(2009, 12, 29, 9, 59, 15, 0, time.UTC)
	if !md.AcquisitionDateTime.Equal(want) {
		t.Errorf("AcquisitionDateTime: got %v, want %v", md.AcquisitionDateTime, want)
	}
}

func TestParseDescriptionRejectsMalformedDate(t *testing.T) {
	desc := "Aperio Image Library v11.2.1\n1x1 (1x1)|Date = garbage|Time = 09:59:15"
	_, err := parseDescription(desc)
	if err == nil {
		t.Fatal("expected error on malformed date")
	}
}
