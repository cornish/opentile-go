package tests

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFixtureSaveLoadRoundTrip exercises both SaveFixture and LoadFixture
// against a tempdir-backed Fixture. Without this, the generate-tagged
// helpers carry zero coverage in the default test suite.
func TestFixtureSaveLoadRoundTrip(t *testing.T) {
	want := &Fixture{
		Slide:  "synthetic.svs",
		Format: "svs",
		Levels: []LevelFixture{
			{Index: 0, Size: [2]int{100, 100}, TileSize: [2]int{16, 16}, Grid: [2]int{7, 7}, Compression: "jpeg", MPPUm: 0.5, PyramidIdx: 0},
			{Index: 1, Size: [2]int{50, 50}, TileSize: [2]int{16, 16}, Grid: [2]int{4, 4}, Compression: "jpeg", MPPUm: 1.0, PyramidIdx: 1},
		},
		TileSHA256: map[string]string{
			"0:0:0": "deadbeef",
			"0:1:0": "cafebabe",
		},
		Metadata: MetadataFixture{Magnification: 20.0, MPP: 0.5},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "fixture.json") // exercises MkdirAll
	if err := SaveFixture(path, want); err != nil {
		t.Fatalf("SaveFixture: %v", err)
	}

	got, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\n  got=%+v\n want=%+v", got, want)
	}
}

// TestLoadFixtureRejectsCorruptJSON guards the parse-error path of
// LoadFixture so an unreadable fixture surfaces a clear error rather than
// returning a zero-valued struct.
func TestLoadFixtureRejectsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.json")
	if err := writeFile(path, []byte("not-json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFixture(path); err == nil {
		t.Fatal("expected error on corrupt JSON")
	}
}

// TestLoadFixtureRejectsMissingFile guards the file-not-found path.
func TestLoadFixtureRejectsMissingFile(t *testing.T) {
	if _, err := LoadFixture("/nonexistent/path/fixture.json"); err == nil {
		t.Fatal("expected error on missing file")
	}
}

// TestTestdataDir round-trips OPENTILE_TESTDIR.
func TestTestdataDir(t *testing.T) {
	t.Setenv("OPENTILE_TESTDIR", "/some/path")
	if got := TestdataDir(); got != "/some/path" {
		t.Errorf("TestdataDir() = %q, want %q", got, "/some/path")
	}
	t.Setenv("OPENTILE_TESTDIR", "")
	if got := TestdataDir(); got != "" {
		t.Errorf("TestdataDir() empty case = %q, want \"\"", got)
	}
}

// TestTileAndSampleKeyFormats locks in the canonical key strings used by
// the fixture JSON; they are persisted, so format drift would silently
// invalidate every fixture in tests/fixtures/.
func TestTileAndSampleKeyFormats(t *testing.T) {
	if got := TileKey(2, 5, 7); got != "2:5:7" {
		t.Errorf("TileKey: got %q, want \"2:5:7\"", got)
	}
	if got := SampleKey(1, SamplePosition{X: 3, Y: 4, Reason: "ignored"}); got != "1:3:4" {
		t.Errorf("SampleKey: got %q, want \"1:3:4\"", got)
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
