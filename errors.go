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
