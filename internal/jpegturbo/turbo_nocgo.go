//go:build !cgo || nocgo

package jpegturbo

// Crop returns ErrCGORequired in nocgo builds. See turbo_cgo.go for the real
// implementation.
func Crop(src []byte, r Region) ([]byte, error) {
	return nil, ErrCGORequired
}

// CropWithBackground returns ErrCGORequired in nocgo builds. See turbo_cgo.go
// for the real implementation.
func CropWithBackground(src []byte, r Region) ([]byte, error) {
	return nil, ErrCGORequired
}
