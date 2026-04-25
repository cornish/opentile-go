// Package tiff parses a minimal subset of the TIFF file format sufficient to
// locate compressed tile byte ranges for whole-slide imaging TIFFs. It is not
// a general-purpose TIFF library; it exposes raw tile bytes and vendor tags
// needed by the opentile-go format packages.
package tiff

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// byteReader reads fixed-width integers at arbitrary offsets in a ReaderAt
// using the byte order established by the TIFF header.
type byteReader struct {
	r     io.ReaderAt
	order binary.ByteOrder
}

func newByteReader(r io.ReaderAt, littleEndian bool) *byteReader {
	order := binary.ByteOrder(binary.BigEndian)
	if littleEndian {
		order = binary.LittleEndian
	}
	return &byteReader{r: r, order: order}
}

func (b *byteReader) read(offset int64, n int) ([]byte, error) {
	buf := make([]byte, n)
	got, err := b.r.ReadAt(buf, offset)
	if err != nil && !(errors.Is(err, io.EOF) && got == n) {
		return nil, fmt.Errorf("tiff: read %d bytes at %d: %w", n, offset, err)
	}
	if got != n {
		return nil, fmt.Errorf("tiff: short read at %d: got %d, want %d", offset, got, n)
	}
	return buf, nil
}

func (b *byteReader) uint16(offset int64) (uint16, error) {
	buf, err := b.read(offset, 2)
	if err != nil {
		return 0, err
	}
	return b.order.Uint16(buf), nil
}

func (b *byteReader) uint32(offset int64) (uint32, error) {
	buf, err := b.read(offset, 4)
	if err != nil {
		return 0, err
	}
	return b.order.Uint32(buf), nil
}

func (b *byteReader) uint64(offset int64) (uint64, error) {
	buf, err := b.read(offset, 8)
	if err != nil {
		return 0, err
	}
	return b.order.Uint64(buf), nil
}

func (b *byteReader) bytes(offset int64, n int) ([]byte, error) {
	return b.read(offset, n)
}
