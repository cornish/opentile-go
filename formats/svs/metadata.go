package svs

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	opentile "github.com/tcornish/opentile-go"
)

// Metadata is the SVS-specific slide metadata. It embeds opentile.Metadata so
// the common fields (magnification, scanner identity, acquisition datetime)
// are populated via the embedded struct; Aperio-specific fields (MPP,
// SoftwareLine, Filename) live on the outer struct.
//
// Consumers read the common fields via opentile.Tiler.Metadata() as usual;
// to read the Aperio-specific fields, pass the Tiler to svs.MetadataOf.
//
// AcquisitionDateTime on the embedded opentile.Metadata carries the Aperio
// Date+Time fields parsed verbatim, with no timezone conversion; Aperio does
// not record a timezone and callers should treat the value as local wall-clock
// time from the scanner.
type Metadata struct {
	opentile.Metadata
	MPP          float64 // microns per pixel
	SoftwareLine string  // first line of ImageDescription
	Filename     string  // Aperio "Filename" key if present
}

// parseDescription decodes the ImageDescription tag stored by Aperio scanners.
// Format: first line is a free-form software banner; subsequent content is
// '|'-separated "key = value" pairs embedded in the same string.
func parseDescription(desc string) (Metadata, error) {
	if !strings.HasPrefix(desc, aperioPrefix) {
		return Metadata{}, errors.New("svs: description is not Aperio")
	}
	var md Metadata

	// Split off the software banner (first line).
	newline := strings.IndexByte(desc, '\n')
	if newline < 0 {
		md.SoftwareLine = desc
		md.ScannerManufacturer = "Aperio"
		md.ScannerSoftware = []string{desc}
		return md, nil
	}
	md.SoftwareLine = desc[:newline]
	md.ScannerManufacturer = "Aperio"
	md.ScannerSoftware = []string{md.SoftwareLine}

	// Parse '|' separated key-value pairs in the remainder.
	body := desc[newline+1:]
	kv := splitKV(body)

	if v, ok := kv["AppMag"]; ok {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return md, fmt.Errorf("svs: parse AppMag %q: %w", v, err)
		}
		md.Magnification = parsed
	}
	if v, ok := kv["MPP"]; ok {
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return md, fmt.Errorf("svs: parse MPP %q: %w", v, err)
		}
		md.MPP = parsed
	}
	if v, ok := kv["ScanScope ID"]; ok {
		md.ScannerSerial = v
	}
	if v, ok := kv["Filename"]; ok {
		md.Filename = v
	}

	// Aperio Date/Time are separate fields in MM/DD/YYYY and HH:MM:SS form.
	date, hasDate := kv["Date"]
	tm, hasTime := kv["Time"]
	if hasDate && hasTime {
		parsed, err := time.Parse("01/02/2006 15:04:05", date+" "+tm)
		if err != nil {
			return md, fmt.Errorf("svs: parse Date/Time %q %q: %w", date, tm, err)
		}
		md.AcquisitionDateTime = parsed
	}
	return md, nil
}

// splitKV parses "k1 = v1|k2 = v2|..." into a map. Whitespace around keys and
// values is trimmed. Tokens without '=' are ignored.
func splitKV(s string) map[string]string {
	out := make(map[string]string)
	for _, tok := range strings.Split(s, "|") {
		eq := strings.IndexByte(tok, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(tok[:eq])
		v := strings.TrimSpace(tok[eq+1:])
		if k != "" {
			out[k] = v
		}
	}
	return out
}
