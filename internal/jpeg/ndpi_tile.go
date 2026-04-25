package jpeg

import (
	"encoding/binary"
	"fmt"
)

// NDPIStripeJPEGHeader parses the JPEG-header prefix embedded at the start of
// an NDPI pyramid-level strip and returns:
//
//   - stripeW, stripeH: the pixel dimensions of one NDPI native stripe. The
//     width is restartInterval × MCU-width, the height is MCU-height (per
//     tifffile's ndpi_jpeg_tile: a single DRI interval covers one MCU row).
//   - patched: a copy of prefix whose SOF0 height/width bytes have been
//     overwritten with (stripeW, stripeH). Callers assemble per-tile JPEGs
//     by concatenating this header with stripe scan fragments and a trailing
//     EOI; the header's on-the-wire dimensions therefore match the assembled
//     bitstream.
//
// This is a direct port of tifffile.ndpi_jpeg_tile (tifffile.py:20991-21064).
// It intentionally mirrors Python's byte layout: output =
//
//	prefix[:sofOffset] + be16(stripeH) + be16(stripeW) + prefix[sofOffset+4:sosOffset]
//
// where sofOffset points at the SOF0 payload's precision byte position + 1
// (the high byte of the height field), and sosOffset is the byte position
// immediately after the SOS segment's length-prefixed payload (i.e. the start
// of scan entropy data — which we drop, so the patched header ends cleanly).
//
// NDPI private tag 65426 (McuStarts) stores the byte offset of each restart
// marker within the strip; the prefix is exactly prefix[:McuStarts[0]] — the
// bytes that come before the first MCU.
func NDPIStripeJPEGHeader(prefix []byte) (stripeW, stripeH int, patched []byte, err error) {
	var (
		restartInterval int
		sofOffset       int
		sosOffset       int
		mcuWidth        int = 1
		mcuHeight       int = 1
	)
	i := 0
	for i < len(prefix) {
		if i+2 > len(prefix) {
			return 0, 0, nil, fmt.Errorf("%w: NDPI prefix truncated at pos %d", ErrBadJPEG, i)
		}
		marker := binary.BigEndian.Uint16(prefix[i : i+2])
		i += 2

		switch {
		case marker == 0xFFD8: // SOI
			continue
		case marker == 0xFFD9: // EOI
			goto done
		case marker >= 0xFFD0 && marker <= 0xFFD7: // RSTn
			continue
		case marker == 0xFF01: // private/unused
			continue
		}

		if i+2 > len(prefix) {
			return 0, 0, nil, fmt.Errorf("%w: NDPI marker 0x%04X missing length at pos %d", ErrBadJPEG, marker, i)
		}
		length := int(binary.BigEndian.Uint16(prefix[i : i+2]))
		i += 2

		switch marker {
		case 0xFFDD: // DRI
			if i+2 > len(prefix) {
				return 0, 0, nil, fmt.Errorf("%w: NDPI DRI truncated at pos %d", ErrBadJPEG, i)
			}
			restartInterval = int(binary.BigEndian.Uint16(prefix[i : i+2]))
		case 0xFFC0: // SOF0
			sofOffset = i + 1
			if i+6 > len(prefix) {
				return 0, 0, nil, fmt.Errorf("%w: NDPI SOF truncated at pos %d", ErrBadJPEG, i)
			}
			// precision, imlength, imwidth, ncomponents
			nComponents := int(prefix[i+5])
			i += 6
			if i+3*nComponents > len(prefix) {
				return 0, 0, nil, fmt.Errorf("%w: NDPI SOF components truncated", ErrBadJPEG)
			}
			mcuWidth = 1
			mcuHeight = 1
			for c := 0; c < nComponents; c++ {
				factor := prefix[i+1]
				i += 3
				if int(factor>>4) > mcuWidth {
					mcuWidth = int(factor >> 4)
				}
				if int(factor&0x0F) > mcuHeight {
					mcuHeight = int(factor & 0x0F)
				}
			}
			mcuWidth *= 8
			mcuHeight *= 8
			// Restore i back to the "length-post" position used by the Python
			// loop's `i += length - 2` convention: sofOffset - 1 is one byte
			// past the length field (i.e. where precision starts).
			i = sofOffset - 1
		case 0xFFDA: // SOS
			sosOffset = i + length - 2
			goto done
		}

		// skip to next marker (length includes the 2 length bytes themselves)
		i += length - 2
	}

done:
	if restartInterval == 0 || sofOffset == 0 || sosOffset == 0 {
		return 0, 0, nil, fmt.Errorf("%w: NDPI prefix missing required JPEG markers (DRI=%d SOF=%d SOS=%d)",
			ErrBadJPEG, restartInterval, sofOffset, sosOffset)
	}
	stripeH = mcuHeight
	stripeW = restartInterval * mcuWidth

	if sofOffset+4 > len(prefix) || sosOffset > len(prefix) || sofOffset+4 > sosOffset {
		return 0, 0, nil, fmt.Errorf("%w: NDPI header offsets out of range (sof=%d sos=%d len=%d)",
			ErrBadJPEG, sofOffset, sosOffset, len(prefix))
	}
	out := make([]byte, 0, sosOffset)
	out = append(out, prefix[:sofOffset]...)
	var sz [4]byte
	binary.BigEndian.PutUint16(sz[0:2], uint16(stripeH))
	binary.BigEndian.PutUint16(sz[2:4], uint16(stripeW))
	out = append(out, sz[:]...)
	out = append(out, prefix[sofOffset+4:sosOffset]...)
	return stripeW, stripeH, out, nil
}
