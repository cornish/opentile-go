package tiff

import "fmt"

// parseBigTIFFHeader reads the 16-byte BigTIFF header starting at offset 2
// (just past the byte-order mark and magic). The byteReader b already has
// the correct endianness set.
//
// BigTIFF layout (TIFF 6.0 Appendix, BigTIFF extension):
//   bytes 0..1   byte order (II/MM) — already consumed
//   bytes 2..3   magic 43          — already consumed
//   bytes 4..5   offset size (must be 8)
//   bytes 6..7   constant (must be 0)
//   bytes 8..15  first IFD offset (uint64)
func parseBigTIFFHeader(b *byteReader, littleEndian bool) (header, error) {
	offsetSize, err := b.uint16(4)
	if err != nil {
		return header{}, fmt.Errorf("%w: BigTIFF offset size: %v", ErrInvalidTIFF, err)
	}
	if offsetSize != 8 {
		return header{}, fmt.Errorf("%w: BigTIFF offset size %d (expected 8)", ErrInvalidTIFF, offsetSize)
	}
	constant, err := b.uint16(6)
	if err != nil {
		return header{}, fmt.Errorf("%w: BigTIFF constant: %v", ErrInvalidTIFF, err)
	}
	if constant != 0 {
		return header{}, fmt.Errorf("%w: BigTIFF constant %d (expected 0)", ErrInvalidTIFF, constant)
	}
	firstIFD, err := b.uint64(8)
	if err != nil {
		return header{}, fmt.Errorf("%w: BigTIFF first IFD: %v", ErrInvalidTIFF, err)
	}
	return header{littleEndian: littleEndian, bigTIFF: true, firstIFD: firstIFD}, nil
}
