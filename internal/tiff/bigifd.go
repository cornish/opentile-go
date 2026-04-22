package tiff

import "fmt"

// walkBigIFDs is the BigTIFF variant of walkIFDs. BigTIFF IFDs use uint64
// entry counts, 20-byte entries (tag u16, type u16, count u64, value u64),
// and uint64 next-IFD offsets.
func walkBigIFDs(b *byteReader, offset int64) ([]*ifd, error) {
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

		count, err := b.uint64(offset)
		if err != nil {
			return nil, fmt.Errorf("tiff: BigTIFF IFD entry count at %d: %w", offset, err)
		}
		ifd := &ifd{entries: make(map[uint16]Entry, count)}
		pos := offset + 8
		for i := uint64(0); i < count; i++ {
			entry, err := readBigEntry(b, pos)
			if err != nil {
				return nil, err
			}
			ifd.entries[entry.Tag] = entry
			pos += 20
		}
		out = append(out, ifd)
		next, err := b.uint64(pos)
		if err != nil {
			return nil, fmt.Errorf("tiff: BigTIFF next IFD offset at %d: %w", pos, err)
		}
		offset = int64(next)
	}
	return out, nil
}

// readBigEntry reads a 20-byte BigTIFF IFD entry at offset.
// Layout: tag(u16) type(u16) count(u64) valueOrOffset(u64).
func readBigEntry(b *byteReader, offset int64) (Entry, error) {
	tag, err := b.uint16(offset)
	if err != nil {
		return Entry{}, err
	}
	typ, err := b.uint16(offset + 2)
	if err != nil {
		return Entry{}, err
	}
	count, err := b.uint64(offset + 4)
	if err != nil {
		return Entry{}, err
	}
	cell, err := b.bytes(offset+12, 8)
	if err != nil {
		return Entry{}, err
	}
	vo := b.order.Uint64(cell)
	var e Entry
	e.Tag = tag
	e.Type = DataType(typ)
	e.Count = count
	e.valueOrOffset = vo
	copy(e.valueBytes[:], cell)
	e.inlineCap = 8
	return e, nil
}
