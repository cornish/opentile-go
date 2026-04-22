package tiff

import (
	"errors"
	"fmt"
	"io"
)

// ErrInvalidTIFF indicates the input does not parse as TIFF at the header level.
// Package-local sentinel; the top-level opentile package wraps it as
// opentile.ErrInvalidTIFF before returning to callers.
var ErrInvalidTIFF = errors.New("tiff: invalid TIFF structure")

// ErrUnsupportedTIFF indicates a valid TIFF variant that opentile-go v0.1 does
// not yet parse (e.g., BigTIFF).
var ErrUnsupportedTIFF = errors.New("tiff: unsupported TIFF variant")

type header struct {
	littleEndian bool
	bigTIFF      bool   // true when magic 43 (BigTIFF); false for magic 42 (classic)
	firstIFD     uint64 // uint64 for BigTIFF; classic TIFF offsets widen safely
}

// parseHeader reads the 8-byte TIFF header (classic) or 16-byte header (BigTIFF).
func parseHeader(r io.ReaderAt) (header, error) {
	var buf [4]byte
	if _, err := r.ReadAt(buf[:], 0); err != nil {
		return header{}, fmt.Errorf("%w: %v", ErrInvalidTIFF, err)
	}
	var le bool
	switch {
	case buf[0] == 'I' && buf[1] == 'I':
		le = true
	case buf[0] == 'M' && buf[1] == 'M':
		le = false
	default:
		return header{}, fmt.Errorf("%w: bad byte order %q", ErrInvalidTIFF, buf[:2])
	}
	b := newByteReader(r, le)
	magic, err := b.uint16(2)
	if err != nil {
		return header{}, fmt.Errorf("%w: %v", ErrInvalidTIFF, err)
	}
	switch magic {
	case 42:
		offset, err := b.uint32(4)
		if err != nil {
			return header{}, fmt.Errorf("%w: %v", ErrInvalidTIFF, err)
		}
		return header{littleEndian: le, bigTIFF: false, firstIFD: uint64(offset)}, nil
	case 43:
		return parseBigTIFFHeader(b, le)
	default:
		return header{}, fmt.Errorf("%w: magic %d", ErrInvalidTIFF, magic)
	}
}
