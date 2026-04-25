package tiff

import "fmt"

// ndpiSourceLensTag is the Hamamatsu vendor tag that identifies NDPI files.
// The tiff package sniffs for it in the first IFD; format packages re-check it
// in their Supports() method. Kept internal to avoid exporting a tag constant
// that's format-specific.
const ndpiSourceLensTag uint16 = 65420

// walkNDPIIFDs walks an NDPI-extended IFD chain. NDPI files use classic TIFF
// magic 42 but embed 64-bit offsets:
//
//   - First-IFD offset in the header is 8 bytes (uint64), which for <4GB
//     files has upper 4 bytes = 0 (compatible with classic 4-byte reading).
//   - Within each IFD: tagno (u16) + 12-byte standard entries + 8-byte
//     next-IFD offset (uint64) + 4 × tagno hi-bits extension.
//   - Each tag's effective valueOrOffset is reconstructed as
//     (hi_bits << 32) | low (the 4-byte inline cell from the standard entry).
func walkNDPIIFDs(b *byteReader, offset int64) ([]*ifd, error) {
	var out []*ifd
	seen := make(map[int64]bool)
	for offset != 0 {
		if len(out) >= maxIFDs {
			return nil, fmt.Errorf("%w (cap=%d)", ErrTooManyIFDs, maxIFDs)
		}
		if seen[offset] {
			return nil, fmt.Errorf("tiff: IFD cycle at offset %d", offset)
		}
		seen[offset] = true

		count16, err := b.uint16(offset)
		if err != nil {
			return nil, fmt.Errorf("tiff: NDPI IFD entry count at %d: %w", offset, err)
		}
		count := uint64(count16)

		ifd := &ifd{entries: make(map[uint16]Entry, count)}

		// Layout:
		//   offset + 2                                    → standard entries (12 * count bytes)
		//   offset + 2 + 12*count                         → next-IFD offset (8 bytes, uint64)
		//   offset + 2 + 12*count + 8                     → hi-bits extension (4 * count bytes)
		entriesBase := offset + 2
		nextIFDOff := entriesBase + int64(12*count)
		hibitsBase := nextIFDOff + 8

		// Read standard entries and high-bits to build Entry values.
		for i := uint64(0); i < count; i++ {
			entryOff := entriesBase + int64(12*i)
			hibitsOff := hibitsBase + int64(4*i)

			tag, err := b.uint16(entryOff)
			if err != nil {
				return nil, err
			}
			typ, err := b.uint16(entryOff + 2)
			if err != nil {
				return nil, err
			}
			countLow, err := b.uint32(entryOff + 4)
			if err != nil {
				return nil, err
			}
			cell, err := b.bytes(entryOff+8, 4)
			if err != nil {
				return nil, err
			}
			valLow := b.order.Uint32(cell)
			valHi, err := b.uint32(hibitsOff)
			if err != nil {
				return nil, err
			}
			fullValue := (uint64(valHi) << 32) | uint64(valLow)

			var e Entry
			e.Tag = tag
			e.Type = DataType(typ)
			e.Count = uint64(countLow)
			e.valueOrOffset = fullValue
			copy(e.valueBytes[:], cell)
			// NDPI inline cell is 4 bytes (the low word of the 8-byte value).
			// The high 4 bytes live in the extension block out-of-band.
			e.inlineCap = 4
			ifd.entries[tag] = e
		}

		out = append(out, ifd)

		// Read next-IFD offset as a single uint64 LE.
		next, err := b.uint64(nextIFDOff)
		if err != nil {
			return nil, fmt.Errorf("tiff: NDPI next IFD at %d: %w", nextIFDOff, err)
		}
		offset = int64(next)
	}
	return out, nil
}

// sniffNDPI parses the first IFD of a classic-TIFF file and reports whether
// it carries the Hamamatsu SourceLens tag (65420), which identifies an NDPI
// file. Used by File.Open to decide whether to re-parse in NDPI mode.
func sniffNDPI(b *byteReader, firstIFDOffset int64) (bool, error) {
	count16, err := b.uint16(firstIFDOffset)
	if err != nil {
		return false, fmt.Errorf("tiff: NDPI sniff count: %w", err)
	}
	for i := uint16(0); i < count16; i++ {
		tag, err := b.uint16(firstIFDOffset + 2 + int64(12*i))
		if err != nil {
			return false, fmt.Errorf("tiff: NDPI sniff tag: %w", err)
		}
		if tag == ndpiSourceLensTag {
			return true, nil
		}
	}
	return false, nil
}
