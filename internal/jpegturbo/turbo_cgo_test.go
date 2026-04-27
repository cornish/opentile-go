//go:build cgo && !nocgo

package jpegturbo

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"sync"
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

func TestCropWithBackgroundInsideImage(t *testing.T) {
	// Fully-inside crop: CropWithBackground should behave like Crop,
	// producing a decoded image of the requested dimensions.
	src := encodeTestJPEG(t, 32, 32)
	got, err := CropWithBackground(src, Region{X: 0, Y: 0, Width: 16, Height: 16})
	if err != nil {
		t.Fatalf("CropWithBackground: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
		t.Errorf("dims: got %v, want 16x16", img.Bounds())
	}
}

func TestCropWithBackgroundBeyondImage(t *testing.T) {
	// Crop extends past the source: Crop would error; CropWithBackground
	// should succeed and return the requested dims. The OOB region is
	// filled in the DCT domain.
	src := encodeTestJPEG(t, 32, 32)
	// Request 48x48 at (0,0) — 16 pixels of OOB on the right and bottom.
	got, err := CropWithBackground(src, Region{X: 0, Y: 0, Width: 48, Height: 48})
	if err != nil {
		t.Fatalf("CropWithBackground OOB: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode OOB: %v", err)
	}
	if img.Bounds().Dx() != 48 || img.Bounds().Dy() != 48 {
		t.Errorf("OOB dims: got %v, want 48x48", img.Bounds())
	}
}

// TestCropWithBackgroundLuminanceWhite verifies the default CropWithBackground
// (luminance=1.0) fills OOB luma with a white-leaning value. We compare the
// luma of a solid-color source pixel (128) against the luma of an OOB pixel
// (fill region) and confirm the fill is brighter.
func TestCropWithBackgroundLuminanceWhite(t *testing.T) {
	src := encodeTestJPEG(t, 32, 32)

	// CropWithBackground defaults to white. The 48x48 crop from (0,0)
	// has a 16x16 in-image region at the top-left (Y=128) and 16 pixels
	// of OOB on each side.
	got, err := CropWithBackground(src, Region{X: 0, Y: 0, Width: 48, Height: 48})
	if err != nil {
		t.Fatalf("CropWithBackground white: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode white: %v", err)
	}

	// Sample a pixel from the right-side OOB column (x=40, y=16 — past
	// the source's right edge at x=32).
	r, g, b, _ := img.At(40, 16).RGBA()
	// RGBA() returns values in [0, 65535]. White = ~65535; gray (128) = ~32896.
	// A white-fill value should be above 50000 in each channel.
	if r < 50000 || g < 50000 || b < 50000 {
		t.Errorf("white OOB pixel at (40,16): RGB=(%d,%d,%d); expected each >= 50000", r>>8, g>>8, b>>8)
	}

	// Also check black fill for contrast.
	got2, err := CropWithBackgroundLuminance(src, Region{X: 0, Y: 0, Width: 48, Height: 48}, 0.0)
	if err != nil {
		t.Fatalf("CropWithBackgroundLuminance 0: %v", err)
	}
	img2, err := jpeg.Decode(bytes.NewReader(got2))
	if err != nil {
		t.Fatalf("decode black: %v", err)
	}
	r2, g2, b2, _ := img2.At(40, 16).RGBA()
	// A black-fill value should be well below the white one — expect each
	// channel below 10000.
	if r2 > 10000 || g2 > 10000 || b2 > 10000 {
		t.Errorf("black OOB pixel at (40,16): RGB=(%d,%d,%d); expected each <= 10000", r2>>8, g2>>8, b2>>8)
	}
}


// TestFillFrameWhite confirms FillFrame with luminance=1.0 produces a
// solid-near-white image. Mirrors the parity check Python opentile's
// JpegFiller.fill_image performs when filling a sparse Philips tile.
func TestFillFrameWhite(t *testing.T) {
	src := encodeTestJPEG(t, 64, 64)
	out, err := FillFrame(src, 1.0)
	if err != nil {
		t.Fatalf("FillFrame: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Errorf("dims: got %v, want 64x64", img.Bounds())
	}
	// Sample several positions; every pixel should be near-white.
	for _, p := range []struct{ x, y int }{{0, 0}, {32, 32}, {63, 63}, {16, 48}} {
		r, g, b, _ := img.At(p.x, p.y).RGBA()
		if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
			t.Errorf("non-white pixel at (%d,%d): RGB=(%d,%d,%d); expected each >= 240",
				p.x, p.y, r>>8, g>>8, b>>8)
		}
	}
}

// TestFillFrameBlack confirms FillFrame with luminance=0.0 produces a
// solid-near-black image.
func TestFillFrameBlack(t *testing.T) {
	src := encodeTestJPEG(t, 32, 32)
	out, err := FillFrame(src, 0.0)
	if err != nil {
		t.Fatalf("FillFrame: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	r, g, b, _ := img.At(16, 16).RGBA()
	if r>>8 > 16 || g>>8 > 16 || b>>8 > 16 {
		t.Errorf("non-black pixel at (16,16): RGB=(%d,%d,%d); expected each <= 16", r>>8, g>>8, b>>8)
	}
}

// TestFillFrameDeterministic locks in the v0.5 T2 gate finding: FillFrame
// with the same input is byte-identical across calls. Sparse-tile parity
// hinges on this.
func TestFillFrameDeterministic(t *testing.T) {
	src := encodeTestJPEG(t, 64, 64)
	a, err := FillFrame(src, 1.0)
	if err != nil {
		t.Fatalf("FillFrame a: %v", err)
	}
	b, err := FillFrame(src, 1.0)
	if err != nil {
		t.Fatalf("FillFrame b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("FillFrame non-deterministic: pass1 %d bytes, pass2 %d bytes", len(a), len(b))
	}
}

// TestCropConcurrentSafe locks in the Crop godoc's safe-for-concurrent-use
// contract: per-call tjInitTransform/tjDestroy means no shared state across
// goroutines, so 32 goroutines × 200 crops should produce byte-identical
// output to a single-threaded baseline. Run with -race to catch any
// inadvertent shared mutation. Closes L9.
func TestCropConcurrentSafe(t *testing.T) {
	src := encodeTestJPEG(t, 64, 64)
	region := Region{X: 0, Y: 0, Width: 16, Height: 16}
	want, err := Crop(src, region)
	if err != nil {
		t.Fatalf("baseline Crop: %v", err)
	}

	const goroutines = 32
	const cropsPerGoroutine = 200
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*cropsPerGoroutine)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for k := 0; k < cropsPerGoroutine; k++ {
				got, err := Crop(src, region)
				if err != nil {
					errs <- fmt.Errorf("goroutine %d crop %d: %w", gid, k, err)
					return
				}
				if !bytes.Equal(got, want) {
					errs <- fmt.Errorf("goroutine %d crop %d: byte mismatch (got %d bytes, want %d)",
						gid, k, len(got), len(want))
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent crop error: %v", err)
	}
}
