//go:build parity

package oracle

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenslideSession is a long-lived openslide-python subprocess scoped
// to one slide. Used for legacy iScan BIF parity (OS-1) where
// openslide reads the file natively. Spec-compliant DP 200 BIFs are
// rejected by openslide ("Bad direction attribute LEFT") and
// validated via the tifffile oracle instead.
//
// Sessions are NOT safe for concurrent use — single-pipe protocol.
type OpenslideSession struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// openslideRunnerScript returns the absolute path to openslide_runner.py,
// resolved next to this source file.
func openslideRunnerScript() string {
	if p := os.Getenv("OPENTILE_OPENSLIDE_RUNNER"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return filepath.Join("tests", "oracle", "openslide_runner.py")
	}
	return filepath.Join(filepath.Dir(thisFile), "openslide_runner.py")
}

// NewOpenslideSession spawns the openslide runner for slide. Reuses
// OPENTILE_ORACLE_PYTHON for the interpreter (or
// OPENTILE_OPENSLIDE_PYTHON if you want to keep openslide-python in a
// different venv from the upstream-opentile parity oracle).
func NewOpenslideSession(slide string) (*OpenslideSession, error) {
	cmd := exec.Command(openslidePythonBin(), openslideRunnerScript(), slide)
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("openslide-oracle: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("openslide-oracle: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("openslide-oracle: start runner: %w", err)
	}
	return &OpenslideSession{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout)}, nil
}

// openslidePythonBin returns the openslide-python interpreter path,
// honoring OPENTILE_OPENSLIDE_PYTHON > OPENTILE_ORACLE_PYTHON > "python3".
func openslidePythonBin() string {
	if p := os.Getenv("OPENTILE_OPENSLIDE_PYTHON"); p != "" {
		return p
	}
	if p := os.Getenv("OPENTILE_ORACLE_PYTHON"); p != "" {
		return p
	}
	return "python3"
}

// CompareResult holds the openslide oracle's verdict on a single
// tile position. Match=true iff every pixel's max channel delta is
// within the threshold the runner uses (currently 4); else
// Match=false and MaxDelta + MismatchCount diagnose the divergence.
//
// Total is the number of pixels compared (tw*th).
//
// OutOfBounds=true means the tile extends past openslide's level
// extent (opentile-go uses the padded TIFF grid; openslide uses the
// AOI hull). Pixel comparison is skipped — mismatch would be
// by-design, not a bug.
type CompareResult struct {
	Match         bool
	DecodeError   bool
	OutOfBounds   bool
	MaxDelta      uint32
	MismatchCount uint32
	Total         uint32
}

// CompareTile uploads jpegBytes to the runner, has it decode via PIL
// (sharing libjpeg-turbo with openslide.read_region), composes the
// openslide reference at (col*tileW, row*tileH) in level coords,
// and returns the per-pixel comparison verdict.
//
// Returns (nil, nil) on a zero-length response (oracle could not run
// — caller should skip this position).
func (s *OpenslideSession) CompareTile(level, col, row, tileW, tileH int, jpegBytes []byte) (*CompareResult, error) {
	header := fmt.Sprintf("compare_tile %d %d %d %d %d %d\n", level, col, row, tileW, tileH, len(jpegBytes))
	if _, err := s.stdin.Write([]byte(header)); err != nil {
		return nil, fmt.Errorf("openslide-oracle: send compare_tile header: %w", err)
	}
	if _, err := s.stdin.Write(jpegBytes); err != nil {
		return nil, fmt.Errorf("openslide-oracle: send compare_tile jpeg payload: %w", err)
	}
	blob, err := s.readBlob()
	if err != nil {
		return nil, err
	}
	if blob == nil {
		return nil, nil
	}
	if len(blob) != 16 {
		return nil, fmt.Errorf("openslide-oracle: compare_tile returned %d bytes, want 16", len(blob))
	}
	status := blob[0]
	mx := binary.BigEndian.Uint32(blob[4:8])
	mc := binary.BigEndian.Uint32(blob[8:12])
	total := binary.BigEndian.Uint32(blob[12:16])
	return &CompareResult{
		Match:         status == 0,
		DecodeError:   status == 2,
		OutOfBounds:   status == 3,
		MaxDelta:      mx,
		MismatchCount: mc,
		Total:         total,
	}, nil
}

// Close sends "quit" and waits for the subprocess to exit.
func (s *OpenslideSession) Close() error {
	_, _ = fmt.Fprintln(s.stdin, "quit")
	_ = s.stdin.Close()
	return s.cmd.Wait()
}

func (s *OpenslideSession) readBlob() ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(s.stdout, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("openslide-oracle: read response length: %w", err)
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(s.stdout, buf); err != nil {
		return nil, fmt.Errorf("openslide-oracle: read response body (%d bytes): %w", length, err)
	}
	return buf, nil
}
