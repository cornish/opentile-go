//go:build cgo && !nocgo

package jpegturbo

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

// encodeTestJPEG creates a plain solid-color JPEG of the given dimensions
// via stdlib — test-only, never imported by library code.
func encodeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewYCbCr(image.Rect(0, 0, w, h), image.YCbCrSubsampleRatio420)
	// Fill Y plane with a constant so MCU alignment is easy to reason about.
	for i := range img.Y {
		img.Y[i] = 128
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestCropMCUAligned(t *testing.T) {
	// 32x32 4:2:0 JPEG → MCU 16x16; crop the top-left 16x16.
	src := encodeTestJPEG(t, 32, 32)
	got, err := Crop(src, Region{X: 0, Y: 0, Width: 16, Height: 16})
	if err != nil {
		t.Fatalf("Crop: %v", err)
	}
	// Decode the crop and confirm dimensions.
	img, err := jpeg.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode cropped: %v", err)
	}
	if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
		t.Errorf("dims: got %v, want 16x16", img.Bounds())
	}
}

func TestCropNonAlignedRejected(t *testing.T) {
	src := encodeTestJPEG(t, 32, 32)
	_, err := Crop(src, Region{X: 1, Y: 0, Width: 16, Height: 16})
	if err == nil {
		t.Fatal("expected error on non-MCU-aligned crop")
	}
}

func TestCropBadInput(t *testing.T) {
	_, err := Crop([]byte("not a jpeg"), Region{X: 0, Y: 0, Width: 8, Height: 8})
	if err == nil {
		t.Fatal("expected error on garbage input")
	}
}
