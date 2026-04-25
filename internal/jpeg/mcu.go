package jpeg

import (
	"bytes"
	"fmt"
)

// MCUSizeOf reads the SOF0 segment of a JPEG byte stream and returns the
// MCU pixel size, derived from each component's sampling factors:
//
//   - YCbCr 4:2:0 (luma 2x2, chroma 1x1): MCU = 16x16
//   - YCbCr 4:2:2 (luma 2x1, chroma 1x1): MCU = 16x8
//   - YCbCr 4:4:4 or grayscale: MCU = 8x8
//
// The returned (w, h) is the maximum sampling factor across components
// multiplied by 8 (the DCT block size).
//
// Errors if SOF0 is missing or if the stream isn't a valid JPEG header.
func MCUSizeOf(jpeg []byte) (w, h int, err error) {
	var sof *SOF
	for seg, scanErr := range Scan(bytes.NewReader(jpeg)) {
		if scanErr != nil {
			return 0, 0, fmt.Errorf("MCUSizeOf scan: %w", scanErr)
		}
		if seg.Marker == SOF0 {
			sof, err = ParseSOF(seg.Payload)
			if err != nil {
				return 0, 0, fmt.Errorf("MCUSizeOf parseSOF: %w", err)
			}
			break
		}
	}
	if sof == nil {
		return 0, 0, fmt.Errorf("%w: SOF0 not found", ErrBadJPEG)
	}
	mw, mh := sof.MCUSize()
	return mw, mh, nil
}
