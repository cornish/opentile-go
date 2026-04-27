package svs_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
)

// TestSVSTileReaderMatchesTile locks in that Level.TileReader returns the
// same bytes as Level.Tile for every level of a real SVS slide. v0.2 had no
// direct test for TileReader; correctness was inferred from the integration
// suite via Tile-only checks.
func TestSVSTileReaderMatchesTile(t *testing.T) {
	dir := os.Getenv("OPENTILE_TESTDIR")
	if dir == "" {
		t.Skip("OPENTILE_TESTDIR not set")
	}
	slide := filepath.Join(dir, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(slide); err != nil {
		t.Skipf("slide not present: %v", err)
	}
	tiler, err := opentile.OpenFile(slide)
	if err != nil {
		t.Fatal(err)
	}
	defer tiler.Close()
	for i, lvl := range tiler.Levels() {
		direct, err := lvl.Tile(0, 0)
		if err != nil {
			t.Errorf("Tile(0,0) level %d: %v", i, err)
			continue
		}
		rc, err := lvl.TileReader(0, 0)
		if err != nil {
			t.Errorf("TileReader(0,0) level %d: %v", i, err)
			continue
		}
		streamed, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Errorf("ReadAll level %d: %v", i, err)
			continue
		}
		if !bytes.Equal(direct, streamed) {
			t.Errorf("level %d: TileReader bytes (%d) != Tile bytes (%d)",
				i, len(streamed), len(direct))
		}
	}
}

// TestSVSTilesIterRowMajor locks in that Level.Tiles yields every (x,y)
// position in row-major order with byte-identical content to Tile(x,y) at
// the same position. Exercised on L0 of CMU-1-Small-Region.svs (12 tiles —
// small enough for a full walk).
func TestSVSTilesIterRowMajor(t *testing.T) {
	dir := os.Getenv("OPENTILE_TESTDIR")
	if dir == "" {
		t.Skip("OPENTILE_TESTDIR not set")
	}
	slide := filepath.Join(dir, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(slide); err != nil {
		t.Skipf("slide not present: %v", err)
	}
	tiler, err := opentile.OpenFile(slide)
	if err != nil {
		t.Fatal(err)
	}
	defer tiler.Close()
	lvl, err := tiler.Level(0)
	if err != nil {
		t.Fatal(err)
	}
	g := lvl.Grid()
	want := make([]opentile.TilePos, 0, g.W*g.H)
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			want = append(want, opentile.TilePos{X: x, Y: y})
		}
	}
	got := make([]opentile.TilePos, 0, len(want))
	for pos, res := range lvl.Tiles(context.Background()) {
		if res.Err != nil {
			t.Errorf("Tiles iter at %v: %v", pos, res.Err)
			continue
		}
		direct, err := lvl.Tile(pos.X, pos.Y)
		if err != nil {
			t.Errorf("Tile(%d,%d): %v", pos.X, pos.Y, err)
			continue
		}
		if !bytes.Equal(direct, res.Bytes) {
			t.Errorf("tile (%d,%d): iter bytes (%d) != Tile bytes (%d)",
				pos.X, pos.Y, len(res.Bytes), len(direct))
		}
		got = append(got, pos)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ordering mismatch: got %d positions (first %v...), want %d (first %v...)",
			len(got), firstN(got, 4), len(want), firstN(want, 4))
	}
}

func firstN(s []opentile.TilePos, n int) []opentile.TilePos {
	if len(s) < n {
		return s
	}
	return s[:n]
}
