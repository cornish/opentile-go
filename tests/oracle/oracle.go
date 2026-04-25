//go:build parity

package oracle

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// PythonBin returns the Python binary to invoke. Honors $OPENTILE_ORACLE_PYTHON
// so developers can point at a pinned venv (e.g. /tmp/opentile-py/bin/python)
// without putting it on PATH. Defaults to "python3".
func PythonBin() string {
	if p := os.Getenv("OPENTILE_ORACLE_PYTHON"); p != "" {
		return p
	}
	return "python3"
}

// RunnerScript returns the absolute path to oracle_runner.py, located in the
// same directory as this source file. Resolved via runtime.Caller so it works
// regardless of the test's working directory. Falls back to
// $OPENTILE_ORACLE_RUNNER if set.
func RunnerScript() string {
	if p := os.Getenv("OPENTILE_ORACLE_RUNNER"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("tests", "oracle", "oracle_runner.py")
	}
	return filepath.Join(filepath.Dir(thisFile), "oracle_runner.py")
}

// Tile invokes Python opentile for a single tile and returns its raw bytes.
// The Python side is configured with OPENTILE_TILE_SIZE = tileSize.
func Tile(slide string, level, x, y, tileSize int) ([]byte, error) {
	cmd := exec.Command(PythonBin(), RunnerScript(), slide, fmt.Sprint(level), fmt.Sprint(x), fmt.Sprint(y))
	cmd.Env = append(os.Environ(), fmt.Sprintf("OPENTILE_TILE_SIZE=%d", tileSize))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python oracle failed (%s): %w\nstderr:\n%s", cmd.Path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// Associated invokes Python opentile for an associated image of the given
// kind ("label", "overview", "thumbnail") and returns its raw bytes.
//
// If Python opentile does not expose that kind on this slide (e.g. NDPI
// CMU-1 has no labels or thumbnails), the runner emits zero-length stdout
// with exit 0; the caller receives a nil/empty slice and should skip the
// comparison. A non-nil error means the subprocess itself failed.
//
// tileSize is passed via OPENTILE_TILE_SIZE for parity with Tile, though
// for associated images it does not affect the returned bytes — the
// associated image is always a single, fixed-size blob.
func Associated(slide, kind string, tileSize int) ([]byte, error) {
	cmd := exec.Command(PythonBin(), RunnerScript(), slide, kind)
	cmd.Env = append(os.Environ(), fmt.Sprintf("OPENTILE_TILE_SIZE=%d", tileSize))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python oracle associated %q failed (%s): %w\nstderr:\n%s", kind, cmd.Path, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
