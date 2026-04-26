package jpeg

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
)

// LumaDCQuant returns the DC quantization coefficient for the luminance
// quantization table (table ID 0) of src. src is a JPEG byte stream; the
// search scans top-down for FF DB (DQT marker), reads the length and
// precision/id nibble, and extracts the DC element (first entry of the
// 64-element zig-zag table).
//
// Direct port of PyTurboJPEG's __find_dqt + __get_dc_dqt_element
// (turbojpeg.py:866-938). Uses Python's naive byte-scan strategy: a DQT
// segment's payload contains quantization values that can in principle
// include the bytes 0xFF 0xDB, so the search is theoretically fragile.
// In practice the first FF DB in a well-formed JPEG is the real DQT
// marker, because DQT appears near the top of the bitstream before SOS
// and entropy data has no raw 0xFF.
//
// Precision: upper nibble of the first payload byte. 0 = 8-bit values
// (signed byte interpretation matches Python's '>b' format spec); 1 =
// 16-bit values ('>h' i.e. signed big-endian int16).
func LumaDCQuant(src []byte) (int, error) {
	return dcQuantForTable(src, 0)
}

// dcQuantForTable finds the DQT with the given table index and returns its
// DC (first) coefficient. Tries a structural Scan-based walk first (which
// cannot conflate FF DB bytes inside other segments' payloads with real
// DQT markers) and falls back to the historical byte-scan if Scan can't
// parse the input — e.g., callers that pass a partial header without SOI
// or with non-standard prefix bytes.
func dcQuantForTable(src []byte, tableIdx int) (int, error) {
	if dc, ok := dcQuantViaScan(src, tableIdx); ok {
		return dc, nil
	}
	return dcQuantViaByteScan(src, tableIdx)
}

// dcQuantViaScan walks src as a JPEG marker sequence and returns the DC
// coefficient for the first DQT entry whose table ID matches tableIdx.
// Returns (0, false) if the structural walk fails for any reason —
// callers should fall back to the byte-scan path.
//
// A single DQT segment can pack multiple tables back-to-back; this
// function walks through them inside one segment payload before moving
// on to the next DQT segment. Mirrors Python opentile's __find_dqt
// outer loop, which also handles multiple-table-per-segment payloads
// (turbojpeg.py:866-938).
func dcQuantViaScan(src []byte, tableIdx int) (int, bool) {
	br := bufio.NewReader(bytes.NewReader(src))
	for seg, err := range Scan(br) {
		if err != nil {
			return 0, false
		}
		if seg.Marker != DQT {
			if seg.Marker == SOS {
				// Past the structural region with no match.
				return 0, false
			}
			continue
		}
		// Walk the segment's tables. Each table starts with a precision/id
		// byte followed by 64 (8-bit) or 128 (16-bit) coefficient bytes.
		p := seg.Payload
		for len(p) >= 1 {
			pid := p[0]
			precision := int(pid >> 4)
			id := int(pid & 0x0F)
			var tableLen int
			switch precision {
			case 0:
				tableLen = 1 + 64
			case 1:
				tableLen = 1 + 128
			default:
				return 0, false
			}
			if len(p) < tableLen {
				return 0, false
			}
			if id == tableIdx {
				switch precision {
				case 0:
					return int(int8(p[1])), true
				case 1:
					return int(int16(binary.BigEndian.Uint16(p[1:3]))), true
				}
			}
			p = p[tableLen:]
		}
	}
	return 0, false
}

// dcQuantViaByteScan is the original v0.2 byte-scan implementation, kept
// as a fallback when Scan fails. Matches Python's bytes.find('\xff\xdb')
// strategy exactly.
func dcQuantViaByteScan(src []byte, tableIdx int) (int, error) {
	pos := 0
	for pos < len(src)-1 {
		// Naive byte-scan for FF DB, matching Python's bytes.find('\xff\xdb').
		found := -1
		for i := pos; i < len(src)-1; i++ {
			if src[i] == 0xFF && src[i+1] == 0xDB {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, fmt.Errorf("%w: DQT with table ID %d not found", ErrBadJPEG, tableIdx)
		}
		dqtStart := found
		// Length at dqtStart+2..+4, precision/id at dqtStart+4.
		if dqtStart+5 > len(src) {
			return 0, fmt.Errorf("%w: DQT truncated at %d", ErrBadJPEG, dqtStart)
		}
		segLen := int(binary.BigEndian.Uint16(src[dqtStart+2 : dqtStart+4]))
		if segLen < 3 {
			return 0, fmt.Errorf("%w: DQT segment length %d invalid", ErrBadJPEG, segLen)
		}
		pid := src[dqtStart+4]
		precision := int(pid >> 4) // 0 = 8-bit, 1 = 16-bit
		id := int(pid & 0x0F)
		if id != tableIdx {
			// Not the table we want; advance past this DQT and continue.
			// Python's loop: `offset += dct_table_offset + dct_table_length`
			// which is equivalent to moving past the whole segment.
			pos = dqtStart + 2 + segLen
			continue
		}
		// DC coefficient is the first element of the zig-zag table; it
		// follows the precision/id byte at dqtStart+5. Python reads it
		// with struct format '>b' (8-bit signed) or '>h' (16-bit signed).
		dcOff := dqtStart + 5
		switch precision {
		case 0:
			if dcOff+1 > len(src) {
				return 0, fmt.Errorf("%w: DQT 8-bit DC truncated", ErrBadJPEG)
			}
			// '>b' signed byte.
			v := int8(src[dcOff])
			return int(v), nil
		case 1:
			if dcOff+2 > len(src) {
				return 0, fmt.Errorf("%w: DQT 16-bit DC truncated", ErrBadJPEG)
			}
			// '>h' signed big-endian int16.
			raw := binary.BigEndian.Uint16(src[dcOff : dcOff+2])
			return int(int16(raw)), nil
		default:
			return 0, fmt.Errorf("%w: DQT precision %d invalid", ErrBadJPEG, precision)
		}
	}
	return 0, fmt.Errorf("%w: DQT with table ID %d not found", ErrBadJPEG, tableIdx)
}

// LuminanceToDCCoefficient maps a luminance level [0,1] to the quantized
// DC DCT coefficient to plant in an out-of-bounds block of a JPEG being
// cropped, such that decoding produces the requested grayscale level.
//
// Direct port of PyTurboJPEG's __map_luminance_to_dc_dct_coefficient
// (turbojpeg.py:941-962). The formula is:
//
//	coef = round((luminance * 2047 - 1024) / dc_dqt)
//
// where dc_dqt is the DC quantization element from the luminance table.
// Pre-quantization DC DCT coefficients span [-1024, 1023]; the formula
// maps luminance to the equivalent post-quantization value.
//
// Luminance is clamped to [0, 1] before the computation, matching
// Python's `min(max(luminance, 0), 1)`.
func LuminanceToDCCoefficient(src []byte, luminance float64) (int, error) {
	if luminance < 0 {
		luminance = 0
	}
	if luminance > 1 {
		luminance = 1
	}
	dcQuant, err := LumaDCQuant(src)
	if err != nil {
		return 0, err
	}
	if dcQuant == 0 {
		return 0, fmt.Errorf("%w: luma DC quant is 0", ErrBadJPEG)
	}
	// Python: int(round((luminance * 2047 - 1024) / dc_dqt)).
	// Python's round() uses banker's rounding (round half to even), which
	// matches math.RoundToEven in Go.
	v := (luminance*2047 - 1024) / float64(dcQuant)
	return int(roundToEvenInt(v)), nil
}

// roundToEvenInt rounds v to the nearest integer with ties going to even,
// matching Python's built-in round() for float inputs.
func roundToEvenInt(v float64) float64 {
	// math.RoundToEven is the direct equivalent.
	return roundToEven(v)
}

// roundToEven mirrors math.RoundToEven without importing the math package
// into a hot path; the logic is short enough to inline.
func roundToEven(v float64) float64 {
	// Handle NaN / +-Inf passively — they round to themselves.
	if v != v {
		return v
	}
	// Positive and negative cases are symmetric; delegate to a helper.
	neg := v < 0
	if neg {
		v = -v
	}
	floor := float64(int64(v))
	frac := v - floor
	var rounded float64
	switch {
	case frac < 0.5:
		rounded = floor
	case frac > 0.5:
		rounded = floor + 1
	default:
		// Tie: round to even.
		if int64(floor)%2 == 0 {
			rounded = floor
		} else {
			rounded = floor + 1
		}
	}
	if neg {
		rounded = -rounded
	}
	return rounded
}
