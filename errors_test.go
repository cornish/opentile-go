package opentile

import (
	"errors"
	"io"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	errs := []error{
		ErrUnsupportedFormat,
		ErrUnsupportedCompression,
		ErrTileOutOfBounds,
		ErrCorruptTile,
		ErrLevelOutOfRange,
		ErrInvalidTIFF,
	}
	seen := make(map[string]bool)
	for _, e := range errs {
		if e == nil {
			t.Fatal("sentinel is nil")
		}
		if seen[e.Error()] {
			t.Errorf("duplicate sentinel text: %q", e.Error())
		}
		seen[e.Error()] = true
	}
}

func TestTileError(t *testing.T) {
	te := &TileError{Level: 2, X: 7, Y: 3, Err: ErrCorruptTile}

	if !errors.Is(te, ErrCorruptTile) {
		t.Fatal("errors.Is should find wrapped sentinel")
	}

	var got *TileError
	if !errors.As(te, &got) {
		t.Fatal("errors.As should extract TileError")
	}
	if got.Level != 2 || got.X != 7 || got.Y != 3 {
		t.Fatalf("TileError fields: got %+v", got)
	}

	wantMsg := "opentile: tile (7,3) on level 2: opentile: corrupt tile"
	if te.Error() != wantMsg {
		t.Fatalf("Error(): got %q, want %q", te.Error(), wantMsg)
	}
}

func TestTileErrorWrapsIO(t *testing.T) {
	te := &TileError{Level: 0, X: 0, Y: 0, Err: io.ErrUnexpectedEOF}
	if !errors.Is(te, io.ErrUnexpectedEOF) {
		t.Fatal("should unwrap to io.ErrUnexpectedEOF")
	}
}
