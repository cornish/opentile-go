package opentile

import "time"

// Metadata is the common subset of slide metadata surfaced across all formats.
// Format packages embed this struct to add format-specific fields exposed via
// type assertion on Tiler.Metadata().
type Metadata struct {
	Magnification       float64   // 0 if unknown
	ScannerManufacturer string
	ScannerModel        string
	ScannerSoftware     []string
	ScannerSerial       string
	// AcquisitionDateTime is the time the slide was scanned. Partial Date
	// or Time values that fail time.Parse yield the zero value
	// (time.Time{}); time.Time{}.IsZero() == true is the "unknown"
	// sentinel. Callers should always check IsZero rather than comparing
	// against a specific time — different scanner vendors emit dates in
	// different formats and our parsers are lenient but not exhaustive.
	AcquisitionDateTime time.Time
}
