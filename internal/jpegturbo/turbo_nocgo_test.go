//go:build !cgo || nocgo

package jpegturbo

import (
	"errors"
	"testing"
)

func TestCropReturnsErrCGORequired(t *testing.T) {
	_, err := Crop([]byte{0xFF, 0xD8, 0xFF, 0xD9}, Region{X: 0, Y: 0, Width: 8, Height: 8})
	if !errors.Is(err, ErrCGORequired) {
		t.Fatalf("expected ErrCGORequired, got %v", err)
	}
}
