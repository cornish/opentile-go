//go:build generate

package tests_test

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	opentile "github.com/tcornish/opentile-go"
	_ "github.com/tcornish/opentile-go/formats/all"
	svs "github.com/tcornish/opentile-go/formats/svs"
	"github.com/tcornish/opentile-go/tests"
)

var regenerate = flag.Bool("generate", false, "regenerate fixtures from live slides")

// TestGenerateFixtures is a dev-only helper. Run with:
//
//	OPENTILE_TESTDIR=$PWD/sample_files/svs \
//	  go test ./tests -tags generate -run TestGenerateFixtures -generate -v
//
// Build tag "generate" keeps it out of default CI.
func TestGenerateFixtures(t *testing.T) {
	if !*regenerate {
		t.Skip("pass -generate to regenerate fixtures")
	}
	dir := tests.TestdataDir()
	if dir == "" {
		t.Fatal("OPENTILE_TESTDIR not set")
	}
	for _, name := range slideCandidates {
		t.Run(name, func(t *testing.T) {
			slide, ok := resolveSlide(dir, name)
			if !ok {
				t.Skipf("slide %s not present under %s", name, dir)
			}
			if err := generateFixture(slide); err != nil {
				t.Fatalf("generate %s: %v", name, err)
			}
		})
	}
}

func generateFixture(slide string) error {
	tiler, err := opentile.OpenFile(slide)
	if err != nil {
		return fmt.Errorf("OpenFile: %w", err)
	}
	defer tiler.Close()

	name := filepath.Base(slide)
	f := &tests.Fixture{
		Slide:      name,
		Format:     string(tiler.Format()),
		TileSHA256: make(map[string]string),
	}
	for i, lvl := range tiler.Levels() {
		f.Levels = append(f.Levels, tests.LevelFixture{
			Index:       i,
			Size:        [2]int{lvl.Size().W, lvl.Size().H},
			TileSize:    [2]int{lvl.TileSize().W, lvl.TileSize().H},
			Grid:        [2]int{lvl.Grid().W, lvl.Grid().H},
			Compression: lvl.Compression().String(),
			MPPUm:       lvl.MPP().W * 1000,
			PyramidIdx:  lvl.PyramidIndex(),
		})
		for y := 0; y < lvl.Grid().H; y++ {
			for x := 0; x < lvl.Grid().W; x++ {
				b, err := lvl.Tile(x, y)
				if err != nil {
					return fmt.Errorf("Tile(%d,%d) level %d: %w", x, y, i, err)
				}
				sum := sha256.Sum256(b)
				f.TileSHA256[tests.TileKey(i, x, y)] = hex.EncodeToString(sum[:])
			}
		}
	}
	for _, a := range tiler.Associated() {
		b, err := a.Bytes()
		if err != nil {
			return fmt.Errorf("Associated(%s).Bytes: %w", a.Kind(), err)
		}
		sum := sha256.Sum256(b)
		f.AssociatedImages = append(f.AssociatedImages, tests.AssociatedFixture{
			Kind:        a.Kind(),
			Size:        [2]int{a.Size().W, a.Size().H},
			Compression: a.Compression().String(),
			SHA256:      hex.EncodeToString(sum[:]),
		})
	}
	md := tiler.Metadata()
	f.Metadata = tests.MetadataFixture{
		Magnification:       md.Magnification,
		ScannerManufacturer: md.ScannerManufacturer,
		ScannerSerial:       md.ScannerSerial,
	}
	if sm, ok := svs.MetadataOf(tiler); ok {
		f.Metadata.SoftwareLine = sm.SoftwareLine
		f.Metadata.MPP = sm.MPP
	}
	if !md.AcquisitionDateTime.IsZero() {
		f.Metadata.AcquisitionRFC3339 = md.AcquisitionDateTime.Format(time.RFC3339)
	}

	outPath := filepath.Join("fixtures", fixtureJSONFor(name))
	if err := tests.SaveFixture(outPath, f); err != nil {
		return fmt.Errorf("SaveFixture: %w", err)
	}
	fmt.Printf("wrote %s\n", outPath)
	return nil
}
