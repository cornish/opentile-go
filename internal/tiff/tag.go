package tiff

import (
	"fmt"
	"strings"
)

// DataType is the TIFF tag data type as defined in TIFF 6.0 + common extensions.
type DataType uint16

const (
	DTByte      DataType = 1
	DTASCII     DataType = 2
	DTShort     DataType = 3
	DTLong      DataType = 4
	DTRational  DataType = 5
	DTUndefined DataType = 7
)

// Size returns the byte size of a single value of the given data type.
// Returns 1 for unknown types so callers reading external offsets can still
// bound-check; callers should check Size() > 0 in a validity guard before use
// for types they care about.
func (d DataType) Size() int {
	switch d {
	case DTByte, DTASCII, DTUndefined:
		return 1
	case DTShort:
		return 2
	case DTLong:
		return 4
	case DTRational:
		return 8
	default:
		return 1
	}
}

// Entry is a raw IFD entry: tag id, type, count, and a 4-byte
// value-or-offset cell. The cell is a little/big-endian encoded uint32 as
// stored in the file; whether it carries the value inline or an external
// offset depends on Count * Type.Size().
type Entry struct {
	Tag           uint16
	Type          DataType
	Count         uint32
	valueOrOffset uint32
	valueBytes    [4]byte // raw 4-byte cell, preserving byte order
}

// fitsInline reports whether the tag value fits in the 4-byte inline cell.
func (e Entry) fitsInline() bool {
	return int64(e.Count)*int64(e.Type.Size()) <= 4
}

// decodeInline decodes the inline 4-byte cell (cell) as a slice of uint32 values
// according to the entry's Type. cell must be the raw 4 bytes in file order.
func (e Entry) decodeInline(b *byteReader, cell []byte) ([]uint32, error) {
	if !e.fitsInline() {
		return nil, fmt.Errorf("tiff: tag %d: value does not fit inline", e.Tag)
	}
	return e.decodeBuffer(b, cell)
}

// decodeExternal decodes values stored at e.valueOrOffset in the underlying file.
func (e Entry) decodeExternal(b *byteReader) ([]uint32, error) {
	n := int64(e.Count) * int64(e.Type.Size())
	buf, err := b.bytes(int64(e.valueOrOffset), int(n))
	if err != nil {
		return nil, fmt.Errorf("tiff: tag %d: %w", e.Tag, err)
	}
	return e.decodeBuffer(b, buf)
}

// Values returns the decoded uint32 values for this entry, reading the inline
// cell when possible or the external offset otherwise.
func (e Entry) Values(b *byteReader) ([]uint32, error) {
	if e.fitsInline() {
		return e.decodeInline(b, e.valueBytes[:])
	}
	return e.decodeExternal(b)
}

// decodeBuffer decodes buf (which must be exactly Count*Type.Size() bytes)
// into uint32 values. Rational and unknown types return raw byte groups as
// uint32s (use dedicated helpers for those cases).
func (e Entry) decodeBuffer(b *byteReader, buf []byte) ([]uint32, error) {
	out := make([]uint32, 0, e.Count)
	switch e.Type {
	case DTByte, DTUndefined:
		for _, v := range buf[:e.Count] {
			out = append(out, uint32(v))
		}
	case DTShort:
		for i := uint32(0); i < e.Count; i++ {
			out = append(out, uint32(b.order.Uint16(buf[i*2:])))
		}
	case DTLong:
		for i := uint32(0); i < e.Count; i++ {
			out = append(out, b.order.Uint32(buf[i*4:]))
		}
	default:
		return nil, fmt.Errorf("tiff: tag %d: unsupported type %d for uint decode", e.Tag, e.Type)
	}
	return out, nil
}

// decodeASCII reads the string value for an ASCII entry.
// cell is the 4-byte inline cell used when the value fits inline.
func (e Entry) decodeASCII(b *byteReader, cell []byte) (string, error) {
	var data []byte
	if e.fitsInline() {
		data = cell[:e.Count]
	} else {
		buf, err := b.bytes(int64(e.valueOrOffset), int(e.Count))
		if err != nil {
			return "", fmt.Errorf("tiff: tag %d: %w", e.Tag, err)
		}
		data = buf
	}
	// TIFF ASCII values are NUL-terminated; strip trailing NULs.
	return strings.TrimRight(string(data), "\x00"), nil
}

// decodeRational reads the uint32 numerator/denominator pairs.
func (e Entry) decodeRational(b *byteReader) ([][2]uint32, error) {
	n := int64(e.Count) * 8
	buf, err := b.bytes(int64(e.valueOrOffset), int(n))
	if err != nil {
		return nil, err
	}
	out := make([][2]uint32, 0, e.Count)
	for i := uint32(0); i < e.Count; i++ {
		num := b.order.Uint32(buf[i*8:])
		den := b.order.Uint32(buf[i*8+4:])
		out = append(out, [2]uint32{num, den})
	}
	return out, nil
}
