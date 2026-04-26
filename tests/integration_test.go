package tests_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	opentile "github.com/tcornish/opentile-go"
	_ "github.com/tcornish/opentile-go/formats/all"
	"github.com/tcornish/opentile-go/tests"
)

// slideCandidates lists SVS and NDPI slides this integration suite knows about.
// Each is tested only if both the on-disk slide and the committed fixture
// JSON are present; otherwise the slide is skipped.
var slideCandidates = []string{
	"CMU-1-Small-Region.svs",
	"CMU-1.svs",
	"JP2K-33003-1.svs",
	"scan_620_.svs",
	"svs_40x_bigtiff.svs",
	"CMU-1.ndpi",
	"OS-2.ndpi",
	"Hamamatsu-1.ndpi",
}

// resolveSlide looks up name in dir, dir/svs, dir/ndpi and returns the first
// existing absolute path. Used so OPENTILE_TESTDIR can be set to the repo
// sample_files root and cover both formats in one run.
func resolveSlide(dir, name string) (string, bool) {
	for _, sub := range []string{"", "svs", "ndpi"} {
		p := filepath.Join(dir, sub, name)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// TestSlideParity reads each candidate slide, walks every (level, x, y), and
// compares against the committed fixture. Slides without a fixture or without
// an on-disk file are skipped — this lets developers iterate on a subset of
// slides without hunting for failures.
func TestSlideParity(t *testing.T) {
	dir := tests.TestdataDir()
	if dir == "" {
		t.Skip("OPENTILE_TESTDIR not set; skipping integration test")
	}
	any := false
	for _, name := range slideCandidates {
		t.Run(name, func(t *testing.T) {
			slide, ok := resolveSlide(dir, name)
			if !ok {
				t.Skipf("slide %s not present under %s", name, dir)
			}
			fixturePath := filepath.Join("fixtures", fixtureJSONFor(name))
			if _, err := os.Stat(fixturePath); err != nil {
				t.Skipf("fixture not present at %s (generate with -generate)", fixturePath)
			}
			any = true
			checkSlideAgainstFixture(t, slide, fixturePath)
		})
	}
	if !any {
		t.Log("no slide+fixture pairs found; run the generator to create fixtures")
	}
}

func checkSlideAgainstFixture(t *testing.T, slide, fixturePath string) {
	t.Helper()
	fix, err := tests.LoadFixture(fixturePath)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	tiler, err := opentile.OpenFile(slide)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer tiler.Close()

	if string(tiler.Format()) != fix.Format {
		t.Errorf("Format: got %q, want %q", tiler.Format(), fix.Format)
	}
	levels := tiler.Levels()
	if len(levels) != len(fix.Levels) {
		t.Fatalf("level count: got %d, want %d", len(levels), len(fix.Levels))
	}
	for i, lvl := range levels {
		exp := fix.Levels[i]
		if lvl.Index() != i {
			t.Errorf("level %d: Index()=%d, want %d", i, lvl.Index(), i)
		}
		if lvl.PyramidIndex() != exp.PyramidIdx {
			t.Errorf("level %d: PyramidIndex()=%d, want %d", i, lvl.PyramidIndex(), exp.PyramidIdx)
		}
		if mpp := lvl.MPP(); mpp.W < 0 || mpp.H < 0 {
			t.Errorf("level %d: MPP negative %v", i, mpp)
		}
		if fp := lvl.FocalPlane(); fp < 0 {
			t.Errorf("level %d: FocalPlane negative %v", i, fp)
		}
		if lvl.Size().W != exp.Size[0] || lvl.Size().H != exp.Size[1] {
			t.Errorf("level %d size: got %v, want %v", i, lvl.Size(), exp.Size)
		}
		if lvl.TileSize().W != exp.TileSize[0] || lvl.TileSize().H != exp.TileSize[1] {
			t.Errorf("level %d tile size: got %v, want %v", i, lvl.TileSize(), exp.TileSize)
		}
		if lvl.Grid().W != exp.Grid[0] || lvl.Grid().H != exp.Grid[1] {
			t.Errorf("level %d grid: got %v, want %v", i, lvl.Grid(), exp.Grid)
		}
		if lvl.Compression().String() != exp.Compression {
			t.Errorf("level %d compression: got %q, want %q", i, lvl.Compression(), exp.Compression)
		}
		// Tile hashes — full-walk mode
		if len(fix.TileSHA256) > 0 {
			for y := 0; y < lvl.Grid().H; y++ {
				for x := 0; x < lvl.Grid().W; x++ {
					b, err := lvl.Tile(x, y)
					if err != nil {
						t.Errorf("Tile(%d,%d) level %d: %v", x, y, i, err)
						continue
					}
					sum := sha256.Sum256(b)
					got := hex.EncodeToString(sum[:])
					key := tests.TileKey(i, x, y)
					want, ok := fix.TileSHA256[key]
					if !ok {
						t.Errorf("fixture missing key %s", key)
						continue
					}
					if got != want {
						t.Errorf("tile %s hash: got %s, want %s", key, got, want)
					}
				}
			}
		}
	}

	// ICCProfile: a non-nil slice must have non-zero length. Some slides
	// legitimately return nil (no embedded profile); only catch the broken
	// case where the tag was found but empty.
	if icc := tiler.ICCProfile(); icc != nil && len(icc) == 0 {
		t.Error("ICCProfile non-nil but empty")
	}

	// Sampled-walk mode: walk only deliberately-chosen positions.
	if len(fix.SampledTileSHA256) > 0 {
		for i, lvl := range levels {
			positions := tests.SamplePositions(lvl.Grid(), lvl.Size(), lvl.TileSize())
			for _, p := range positions {
				b, err := lvl.Tile(p.X, p.Y)
				if err != nil {
					t.Errorf("sampled Tile(%d,%d) level %d: %v", p.X, p.Y, i, err)
					continue
				}
				key := tests.SampleKey(i, p)
				expEntry, ok := fix.SampledTileSHA256[key]
				if !ok {
					t.Errorf("sampled fixture missing key %s", key)
					continue
				}
				sum := sha256.Sum256(b)
				got := hex.EncodeToString(sum[:])
				if got != expEntry.SHA256 {
					t.Errorf("sampled tile %s (%s): got %s, want %s",
						key, expEntry.Reason, got, expEntry.SHA256)
				}
			}
		}
	}

	md := tiler.Metadata()
	if md.Magnification != fix.Metadata.Magnification {
		t.Errorf("magnification: got %v, want %v", md.Magnification, fix.Metadata.Magnification)
	}

	associated := tiler.Associated()
	if len(associated) != len(fix.AssociatedImages) {
		t.Errorf("associated count: got %d, want %d", len(associated), len(fix.AssociatedImages))
	} else {
		for i, a := range associated {
			exp := fix.AssociatedImages[i]
			if a.Kind() != exp.Kind {
				t.Errorf("associated[%d] kind: got %q, want %q", i, a.Kind(), exp.Kind)
			}
			if a.Compression().String() != exp.Compression {
				t.Errorf("associated[%d] compression: got %q, want %q", i, a.Compression(), exp.Compression)
			}
			if a.Size().W != exp.Size[0] || a.Size().H != exp.Size[1] {
				t.Errorf("associated[%d] size: got %v, want %v", i, a.Size(), exp.Size)
			}
			b, err := a.Bytes()
			if err != nil {
				t.Errorf("associated[%d] Bytes: %v", i, err)
				continue
			}
			sum := sha256.Sum256(b)
			if got := hex.EncodeToString(sum[:]); got != exp.SHA256 {
				t.Errorf("associated[%d] sha256: got %s, want %s", i, got, exp.SHA256)
			}
		}
	}
}

// fixtureJSONFor returns the fixture filename for a given slide filename.
// SVS slides keep the historical "<stem>.json" naming. NDPI slides embed the
// ".ndpi" extension as "<stem>.ndpi.json" so that (for example) CMU-1.svs and
// CMU-1.ndpi produce distinct fixtures on disk.
func fixtureJSONFor(slideFilename string) string {
	base := filepath.Base(slideFilename)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if ext == ".ndpi" {
		return stem + ".ndpi.json"
	}
	return stem + ".json"
}

// Silence the imports when the test file is compiled with no tests run.
var _ = fmt.Sprintf
