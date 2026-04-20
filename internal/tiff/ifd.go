package tiff

import (
	"fmt"
)

// ifd is a parsed Image File Directory: a collection of Entry values indexed by tag id.
type ifd struct {
	entries map[uint16]Entry
}

func (i *ifd) get(tag uint16) (Entry, bool) {
	e, ok := i.entries[tag]
	return e, ok
}

// maxIFDs guards against pathological inputs with circular or excessive IFD chains.
const maxIFDs = 1024

// walkIFDs reads the IFD chain starting at offset and returns every IFD.
// The chain terminates when next-IFD-offset is zero or maxIFDs is reached
// (returning an error in the latter case).
func walkIFDs(b *byteReader, offset int64) ([]*ifd, error) {
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

		count, err := b.uint16(offset)
		if err != nil {
			return nil, fmt.Errorf("tiff: IFD entry count at %d: %w", offset, err)
		}
		ifd := &ifd{entries: make(map[uint16]Entry, count)}
		pos := offset + 2
		for i := uint16(0); i < count; i++ {
			entry, err := readEntry(b, pos)
			if err != nil {
				return nil, err
			}
			ifd.entries[entry.Tag] = entry
			pos += 12
		}
		out = append(out, ifd)
		next, err := b.uint32(pos)
		if err != nil {
			return nil, fmt.Errorf("tiff: next IFD offset at %d: %w", pos, err)
		}
		offset = int64(next)
	}
	return out, nil
}

// readEntry reads a 12-byte IFD entry at offset.
func readEntry(b *byteReader, offset int64) (Entry, error) {
	tag, err := b.uint16(offset)
	if err != nil {
		return Entry{}, err
	}
	typ, err := b.uint16(offset + 2)
	if err != nil {
		return Entry{}, err
	}
	count, err := b.uint32(offset + 4)
	if err != nil {
		return Entry{}, err
	}
	cell, err := b.bytes(offset+8, 4)
	if err != nil {
		return Entry{}, err
	}
	vo := b.order.Uint32(cell)
	var e Entry
	e.Tag = tag
	e.Type = DataType(typ)
	e.Count = count
	e.valueOrOffset = vo
	copy(e.valueBytes[:], cell)
	return e, nil
}
