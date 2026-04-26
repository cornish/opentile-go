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

// CropWithBackgroundLuminance returns ErrCGORequired in nocgo builds. See
// turbo_cgo.go for the real implementation.
func CropWithBackgroundLuminance(src []byte, r Region, luminance BackgroundLuminance) ([]byte, error) {
	return nil, ErrCGORequired
}

// CropWithBackgroundLuminanceOpts returns ErrCGORequired in nocgo builds.
// See turbo_cgo.go for the real implementation.
func CropWithBackgroundLuminanceOpts(src []byte, r Region, luminance BackgroundLuminance, opts CropOpts) ([]byte, error) {
	return nil, ErrCGORequired
}
