// Package tests contains integration test helpers and fixture schemas shared
// between integration and fixture-generation tests.
package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Fixture is the on-disk schema for a single-slide parity fixture.
type Fixture struct {
	Slide            string            `json:"slide"`
	Format           string            `json:"format"`
	Levels           []LevelFixture    `json:"levels"`
	Metadata         MetadataFixture   `json:"metadata"`
	TileSHA256       map[string]string `json:"tiles"` // key: "level:x:y", value: hex sha256
	ICCProfileSHA256 string            `json:"icc_profile_sha256,omitempty"`
}

type LevelFixture struct {
	Index       int     `json:"index"`
	Size        [2]int  `json:"size"`
	TileSize    [2]int  `json:"tile_size"`
	Grid        [2]int  `json:"grid"`
	Compression string  `json:"compression"`
	MPPUm       float64 `json:"mpp_um"`
	PyramidIdx  int     `json:"pyramid_index"`
}

type MetadataFixture struct {
	Magnification       float64 `json:"magnification"`
	ScannerManufacturer string  `json:"scanner_manufacturer,omitempty"`
	ScannerSerial       string  `json:"scanner_serial,omitempty"`
	SoftwareLine        string  `json:"software_line,omitempty"`
	MPP                 float64 `json:"mpp_um,omitempty"`
	AcquisitionRFC3339  string  `json:"acquisition_rfc3339,omitempty"`
}

// LoadFixture reads a Fixture from fixturePath.
func LoadFixture(fixturePath string) (*Fixture, error) {
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, err
	}
	f := &Fixture{}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, fmt.Errorf("parse fixture %s: %w", fixturePath, err)
	}
	return f, nil
}

// SaveFixture writes f to fixturePath as indented JSON.
func SaveFixture(fixturePath string, f *Fixture) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fixturePath, data, 0o644)
}

// TileKey formats a (level, x, y) triple for fixture lookup.
func TileKey(level, x, y int) string {
	return fmt.Sprintf("%d:%d:%d", level, x, y)
}

// TestdataDir returns the directory holding slide files for integration tests.
// Resolved from OPENTILE_TESTDIR env var; empty string means integration tests
// should t.Skip.
func TestdataDir() string { return os.Getenv("OPENTILE_TESTDIR") }
