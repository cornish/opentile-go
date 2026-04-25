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
	AcquisitionDateTime time.Time // zero if unknown
}
