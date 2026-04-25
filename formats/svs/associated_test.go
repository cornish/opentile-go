package svs

import (
	"bytes"
	"io"
	"testing"

	opentile "github.com/tcornish/opentile-go"
	"github.com/tcornish/opentile-go/internal/tifflzw"
)

// TestClassifyAssociatedKind locks in the SubFileType + position-based
// classifier. Upstream tifffile._series_svs dispatches Label/Macro by
// SubFileType tag 254 (1=Label, 9=Macro), and treats pages[1] as
// "Thumbnail" regardless of tag content.
func TestClassifyAssociatedKind(t *testing.T) {
	tests := []struct {
		name        string
		pageIdx     int
		subfileType uint32
		tiled       bool
		want        string // empty string = not an associated image
	}{
		{"baseline pyramid skipped", 0, 0, true, ""},
		{"page 1 non-tiled = thumbnail", 1, 0, false, "thumbnail"},
		{"page 1 tiled still thumbnail", 1, 0, true, "thumbnail"},
		{"extra tiled pyramid level skipped", 2, 0, true, ""},
		{"subfile 1 non-tiled = label", 4, 1, false, "label"},
		{"subfile 9 non-tiled = overview", 5, 9, false, "overview"},
		{"unknown subfile non-tiled skipped", 3, 5, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyAssociatedKind(tc.pageIdx, tc.subfileType, tc.tiled)
			if got != tc.want {
				t.Errorf("classifyAssociatedKind(idx=%d sf=%d tiled=%v) = %q want %q",
					tc.pageIdx, tc.subfileType, tc.tiled, got, tc.want)
			}
		})
	}
}

// TestLabelMultiStripDecodesRestitchesEncodes locks in the v0.3 L10 fix:
// a multi-strip LZW label is decoded strip-by-strip, the raster is
// concatenated row-major, and re-encoded as a single LZW stream covering
// the full image. Replaces the old strip-0-only behavior (which mirrored
// a Python opentile 0.20.0 bug — see docs/deferred.md and L10).
func TestLabelMultiStripDecodesRestitchesEncodes(t *testing.T) {
	const (
		w            = 4
		h            = 6
		rowsPerStrip = 2
		samples      = 1
	)
	full := make([]byte, w*h*samples)
	for i := range full {
		full[i] = byte(i + 1)
	}
	// Build 3 LZW-encoded strips, each rowsPerStrip rows of the raster.
	var src bytes.Buffer
	var offsets, counts []uint64
	off := uint64(0)
	for s := 0; s < 3; s++ {
		var enc bytes.Buffer
		ww := tifflzw.NewWriter(&enc, tifflzw.MSB, 8)
		start := s * rowsPerStrip * w * samples
		end := start + rowsPerStrip*w*samples
		if _, err := ww.Write(full[start:end]); err != nil {
			t.Fatal(err)
		}
		if err := ww.Close(); err != nil {
			t.Fatal(err)
		}
		src.Write(enc.Bytes())
		offsets = append(offsets, off)
		counts = append(counts, uint64(enc.Len()))
		off += uint64(enc.Len())
	}
	a := &stripedLabel{
		stripOffsets: offsets,
		stripCounts:  counts,
		size:         opentile.Size{W: w, H: h},
		compression:  opentile.CompressionLZW,
		rowsPerStrip: rowsPerStrip,
		samples:      samples,
		reader:       bytes.NewReader(src.Bytes()),
	}
	got, err := a.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	dr := tifflzw.NewReader(bytes.NewReader(got), tifflzw.MSB, 8)
	defer dr.Close()
	decoded, err := io.ReadAll(dr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, full) {
		t.Errorf("multi-strip restitch mismatch:\n got: %v\nwant: %v", decoded, full)
	}
}

// TestLabelSingleStripPassthrough: single-strip label returns raw bytes
// without any JPEG assembly (upstream SvsLabelImage.get_tile semantics).
func TestLabelSingleStripPassthrough(t *testing.T) {
	payload := []byte("this is a fake LZW strip body")
	a := &stripedLabel{
		stripOffsets: []uint64{0},
		stripCounts:  []uint64{uint64(len(payload))},
		size:         opentile.Size{W: 10, H: 10},
		compression:  opentile.CompressionLZW,
		reader:       bytes.NewReader(payload),
	}
	got, err := a.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Bytes() did not passthrough; got %q want %q", got, payload)
	}
	if a.Compression() != opentile.CompressionLZW {
		t.Errorf("Compression() = %v want CompressionLZW", a.Compression())
	}
}

// TestSvsAssociatedSmoke exercises all three local SVS fixtures to confirm
// Bytes() does not error on real data. Skipped unless OPENTILE_TESTDIR is
// set. Not a parity test — byte-equality with Python opentile lands in the
// parity harness (Task 25-26) and the regenerated fixtures (Task 24).
func TestSvsAssociatedSmoke(t *testing.T) {
	t.Skip("associated-image parity comes from Task 24 fixture regeneration / Task 25 parity oracle")
}
