package tiff

import "fmt"

// ndpiSourceLensTag is the Hamamatsu vendor tag that identifies NDPI files.
// The tiff package sniffs for it in the first IFD; format packages re-check it
// in their Supports() method. Kept internal to avoid exporting a tag constant
// that's format-specific.
const ndpiSourceLensTag uint16 = 65420

// walkNDPIIFDs walks an NDPI-extended IFD chain. NDPI files use classic TIFF
// magic 42 but embed 64-bit offsets via a per-tag 4-byte high-bits extension
// block placed after the standard 12-byte tag entries (with an 8-byte
// padding block between).
//
// Per-IFD layout:
//
//	tagno   : uint16
//	entries : 12 bytes * tagno   (standard classic entry: tag u16, type u16, count u32, valueOrOffset u32)
//	padding : 8 bytes
//	hibits  : 4 bytes * tagno    (high 32 bits of valueOrOffset)
//	nextLow : uint32
//	nextHi  : uint32
//
// For each tag, the full 64-bit valueOrOffset is reconstructed as
// (hibits << 32) | low.
func walkNDPIIFDs(b *byteReader, offset int64) ([]*ifd, error) {
	var out []*ifd
	seen := make(map[int64]bool)
	for offset != 0 {
		if len(out) >= maxIFDs {
			return nil, fmt.Errorf("tiff: IFD chain exceeds max length %d", maxIFDs)
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

		cur := &ifd{entries: make(map[uint16]Entry, count)}
		// Standard entries start at offset + 2.
		entriesBase := offset + 2
		// High-bits block starts at offset + 2 + 12*count + 8 (8-byte padding).
		hibitsBase := entriesBase + int64(12*count) + 8

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
			// Store the full reconstructed 64-bit value in valueBytes so that
			// Values64 (which reads valueBytes when fitsInline) returns the
			// correct high bits. valueBytes is [8]byte, so this is safe.
			// Values (uint32) reads valueBytes[:4] which yields the low 32 bits
			// — correct for inline scalar values that fit in uint32.
			b.order.PutUint64(e.valueBytes[:], fullValue)
			// NDPI's inline cell is still 4 bytes (the 32-bit low word of the
			// value); the high 4 bytes live out-of-band in the extension block.
			// Mark inlineCap=4.
			e.inlineCap = 4
			cur.entries[tag] = e
		}

		out = append(out, cur)

		// Next-IFD offset: 4 low + 4 high immediately after the high-bits
		// block for this IFD.
		nextLowOff := hibitsBase + int64(4*count)
		nextLow, err := b.uint32(nextLowOff)
		if err != nil {
			return nil, fmt.Errorf("tiff: NDPI next IFD low: %w", err)
		}
		nextHi, err := b.uint32(nextLowOff + 4)
		if err != nil {
			return nil, fmt.Errorf("tiff: NDPI next IFD high: %w", err)
		}
		offset = int64((uint64(nextHi) << 32) | uint64(nextLow))
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
