package opentile

import (
	"errors"
	"fmt"
)

var (
	ErrUnsupportedFormat      = errors.New("opentile: unsupported format")
	ErrUnsupportedCompression = errors.New("opentile: unsupported compression")
	ErrTileOutOfBounds        = errors.New("opentile: tile position out of bounds")
	ErrCorruptTile            = errors.New("opentile: corrupt tile")
	ErrLevelOutOfRange        = errors.New("opentile: level index out of range")
	ErrInvalidTIFF            = errors.New("opentile: invalid TIFF structure")

	// Returned (wrapped in TileError) when internal/jpeg cannot parse a JPEG
	// bitstream or assemble a valid one from TIFF fragments.
	ErrBadJPEGBitstream = errors.New("opentile: invalid JPEG bitstream")

	// Returned when an operation requires an MCU-aligned region and the
	// computed or requested region is not. Primarily an internal invariant
	// guard; consumers encounter it only on malformed slides.
	ErrMCUAlignment = errors.New("opentile: operation requires MCU alignment")

	// Returned from NDPI one-frame levels and NDPI label on builds compiled
	// without cgo (CGO_ENABLED=0 or -tags nocgo).
	ErrCGORequired = errors.New("opentile: operation requires cgo build with libjpeg-turbo")

	// Reserved for future use; currently unfired because v0.2 defaults the
	// NDPI tile size to 512 rather than erroring. Predefined so exporting
	// it later is not a breaking change.
	ErrTileSizeRequired = errors.New("opentile: tile size not representable for this format")
)

// TileError wraps a per-tile failure with the (level, x, y) that produced it.
// Consumers use errors.As to extract the coordinates and errors.Is against the
// exported sentinels to branch on the underlying cause.
type TileError struct {
	Level int
	X, Y  int
	Err   error
}

func (e *TileError) Error() string {
	return fmt.Sprintf("opentile: tile (%d,%d) on level %d: %v", e.X, e.Y, e.Level, e.Err)
}

func (e *TileError) Unwrap() error { return e.Err }
