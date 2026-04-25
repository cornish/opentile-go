// Package jpegturbo provides a minimal cgo wrapper over libjpeg-turbo's
// tjTransform operation, scoped to the lossless MCU-aligned JPEG crop that
// opentile-go needs for one-frame NDPI pyramid levels and NDPI label
// cropping. It is deliberately the only cgo package in the module.
//
// Default builds link libjpeg-turbo 2.1+ via pkg-config. The `nocgo` build
// tag (or CGO_ENABLED=0) swaps in a stub Crop that returns ErrCGORequired,
// letting the rest of the library build and run for SVS-only consumers who
// cannot link C dependencies.
package jpegturbo

import "errors"

// ErrCGORequired is returned from Crop when the package was compiled without
// cgo support. Callers propagate this wrapped in opentile.TileError.
var ErrCGORequired = errors.New("jpegturbo: this operation requires cgo + libjpeg-turbo (build without -tags nocgo)")

// Region is an MCU-aligned pixel rectangle within a JPEG. libjpeg-turbo with
// TJXOPT_PERFECT rejects non-aligned regions rather than silently producing
// a partial MCU output.
type Region struct {
	X, Y, Width, Height int
}

// BackgroundLuminance is a [0,1] fill level for out-of-bounds DCT blocks
// when CropWithBackground's crop region extends past the source image.
// 0.0 = black, 1.0 = white, 0.5 = mid-gray. The default when zero-valued
// must be 1.0 to match Python opentile's PyTurboJPEG.crop_multiple default.
type BackgroundLuminance float64

// DefaultBackgroundLuminance is Python opentile's default: white fill,
// matching PyTurboJPEG.crop_multiple's background_luminance=1.0 argument.
const DefaultBackgroundLuminance BackgroundLuminance = 1.0
