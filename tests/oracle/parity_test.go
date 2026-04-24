//go:build parity

package oracle_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	opentile "github.com/tcornish/opentile-go"
	_ "github.com/tcornish/opentile-go/formats/all"
	"github.com/tcornish/opentile-go/tests"
	"github.com/tcornish/opentile-go/tests/oracle"
)

var fullParity = flag.Bool("parity-full", false, "walk every tile (slow) instead of sampling up to 10 positions per level")

var slideCandidates = []string{
	"CMU-1-Small-Region.svs",
	"CMU-1.svs",
	"JP2K-33003-1.svs",
	"CMU-1.ndpi",
	"OS-2.ndpi",
}

// tileSize is the output tile size both Go and Python use for parity. Keep
// these in lockstep: WithTileSize on the Go side and OPENTILE_TILE_SIZE on
// the Python side (set by oracle.Tile). Use 1024 matching the upstream
// Python opentile default.
const tileSize = 1024

func TestParityAgainstPython(t *testing.T) {
	dir := tests.TestdataDir()
	if dir == "" {
		t.Skip("OPENTILE_TESTDIR not set; skipping parity test")
	}
	for _, name := range slideCandidates {
		t.Run(name, func(t *testing.T) {
			slide, ok := resolveSlide(dir, name)
			if !ok {
				t.Skipf("slide %s not present under %s", name, dir)
			}
			runParityOnSlide(t, slide)
		})
	}
}

func runParityOnSlide(t *testing.T, slide string) {
	tiler, err := opentile.OpenFile(slide, opentile.WithTileSize(tileSize, tileSize))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tiler.Close()
	isNDPI := strings.EqualFold(filepath.Ext(slide), ".ndpi")
	for li, lvl := range tiler.Levels() {
		positions := samplePositions(lvl.Grid(), *fullParity)
		imgSize := lvl.Size()
		for _, pos := range positions {
			our, err := lvl.Tile(pos.X, pos.Y)
			if err != nil {
				t.Errorf("level %d tile (%d,%d): Go error: %v", li, pos.X, pos.Y, err)
				continue
			}
			theirs, err := oracle.Tile(slide, li, pos.X, pos.Y, tileSize)
			if err != nil {
				t.Errorf("level %d tile (%d,%d): Python oracle error: %v", li, pos.X, pos.Y, err)
				continue
			}
			if !bytes.Equal(our, theirs) {
				// Edge tile = last column or last row (pixel extent exceeds image bounds).
				isEdge := (pos.X+1)*tileSize > imgSize.W || (pos.Y+1)*tileSize > imgSize.H
				if isNDPI && isEdge {
					t.Logf("slide %s level %d tile (%d,%d): edge-tile divergence accepted under L12 (go=%d bytes, py=%d bytes) — see docs/deferred.md L12",
						filepath.Base(slide), li, pos.X, pos.Y, len(our), len(theirs))
					continue
				}
				t.Errorf("slide %s level %d tile (%d,%d): byte-level divergence (go=%d bytes, py=%d bytes)",
					filepath.Base(slide), li, pos.X, pos.Y, len(our), len(theirs))
			}
		}
	}
}

// samplePositions returns a reproducible set of tile positions covering the
// corners, edges, and interior, clamped to grid bounds. Full mode enumerates
// every position — opt in with -parity-full; it's minutes per slide, hours
// for OS-2.ndpi.
func samplePositions(grid opentile.Size, full bool) []opentile.TilePos {
	if full {
		out := make([]opentile.TilePos, 0, grid.W*grid.H)
		for y := 0; y < grid.H; y++ {
			for x := 0; x < grid.W; x++ {
				out = append(out, opentile.TilePos{X: x, Y: y})
			}
		}
		return out
	}
	cand := []opentile.TilePos{
		{X: 0, Y: 0},
		{X: grid.W - 1, Y: 0},
		{X: 0, Y: grid.H - 1},
		{X: grid.W - 1, Y: grid.H - 1},
		{X: grid.W / 4, Y: grid.H / 4},
		{X: grid.W / 2, Y: grid.H / 2},
		{X: 3 * grid.W / 4, Y: 3 * grid.H / 4},
		{X: 1, Y: grid.H / 2},
		{X: grid.W / 2, Y: 1},
		{X: grid.W - 2, Y: grid.H - 2},
	}
	seen := make(map[opentile.TilePos]bool)
	out := cand[:0]
	for _, p := range cand {
		if p.X < 0 || p.Y < 0 || p.X >= grid.W || p.Y >= grid.H {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func resolveSlide(dir, name string) (string, bool) {
	for _, sub := range []string{"", "svs", "ndpi"} {
		p := filepath.Join(dir, sub, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}
