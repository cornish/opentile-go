package bif

import (
	"bytes"
	"image/jpeg"
	"testing"
)

func TestBlankTileBasic(t *testing.T) {
	b, err := blankTile(1024, 1024, 235)
	if err != nil {
		t.Fatalf("blankTile(1024, 1024, 235) failed: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("blankTile returned empty bytes")
	}
	// Check JPEG SOI and EOI markers
	if b[0] != 0xFF || b[1] != 0xD8 {
		t.Errorf("expected SOI marker (FF D8), got %02X %02X", b[0], b[1])
	}
	if b[len(b)-2] != 0xFF || b[len(b)-1] != 0xD9 {
		t.Errorf("expected EOI marker (FF D9), got %02X %02X", b[len(b)-2], b[len(b)-1])
	}
}

func TestBlankTileCacheSameKey(t *testing.T) {
	b1, err := blankTile(1024, 1024, 235)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	b2, err := blankTile(1024, 1024, 235)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	// Must be the exact same slice (pointer-identical)
	if &b1[0] != &b2[0] {
		t.Error("expected cache hit: returned slices have different backing arrays")
	}
}

func TestBlankTileDifferentWhite(t *testing.T) {
	b235, err := blankTile(1024, 1024, 235)
	if err != nil {
		t.Fatalf("blankTile(..., 235) failed: %v", err)
	}
	b255, err := blankTile(1024, 1024, 255)
	if err != nil {
		t.Fatalf("blankTile(..., 255) failed: %v", err)
	}
	if bytes.Equal(b235, b255) {
		t.Error("expected different tiles for different white values")
	}
}

func TestBlankTileDifferentDimensions(t *testing.T) {
	b1024x1024, err := blankTile(1024, 1024, 235)
	if err != nil {
		t.Fatalf("blankTile(1024, 1024, ...) failed: %v", err)
	}
	b1024x1360, err := blankTile(1024, 1360, 235)
	if err != nil {
		t.Fatalf("blankTile(1024, 1360, ...) failed: %v", err)
	}
	if bytes.Equal(b1024x1024, b1024x1360) {
		t.Error("expected different tiles for different dimensions")
	}
}

func TestBlankTileDecoded(t *testing.T) {
	b, err := blankTile(1024, 1024, 235)
	if err != nil {
		t.Fatalf("blankTile failed: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("jpeg.Decode failed: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1024 || bounds.Dy() != 1024 {
		t.Errorf("expected 1024x1024, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// Sample 4 corners + 1 center pixel; allow ±2 tolerance for JPEG round-trip
	samples := []struct {
		x, y int
		name string
	}{
		{0, 0, "top-left"},
		{1023, 0, "top-right"},
		{0, 1023, "bottom-left"},
		{1023, 1023, "bottom-right"},
		{512, 512, "center"},
	}

	for _, s := range samples {
		r, g, b, a := img.At(s.x, s.y).RGBA()
		// RGBA returns 16-bit values; scale to 8-bit
		r8 := uint8(r >> 8)
		g8 := uint8(g >> 8)
		b8 := uint8(b >> 8)
		a8 := uint8(a >> 8)

		// All channels should be close to 235 (within ±2 tolerance)
		const tolerance = 2
		if !(r8 >= 233 && r8 <= 237) || !(g8 >= 233 && g8 <= 237) || !(b8 >= 233 && b8 <= 237) {
			t.Errorf("%s: expected ≈235, got R=%d G=%d B=%d", s.name, r8, g8, b8)
		}
		if a8 != 255 {
			t.Errorf("%s: expected A=255, got A=%d", s.name, a8)
		}
	}
}

func TestBlankTileInvalidDimensions(t *testing.T) {
	tests := []struct {
		w, h int
		name string
	}{
		{0, 1024, "zero width"},
		{-1, 1024, "negative width"},
		{1024, 0, "zero height"},
		{1024, -1, "negative height"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := blankTile(tc.w, tc.h, 235)
			if err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}
