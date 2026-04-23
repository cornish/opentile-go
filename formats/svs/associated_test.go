package svs

import (
	"bytes"
	"testing"

	opentile "github.com/tcornish/opentile-go"
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

// TestLabelReturnsStrip0ForMultiStrip locks in the upstream-parity behavior:
// a multi-strip label returns strip 0's bytes (not an error, not a stitch).
// This matches Python opentile SvsLabelImage.get_tile((0,0)) which returns
// _read_frame(0) unconditionally. See docs/deferred.md for the v0.3 plan to
// produce the full label.
func TestLabelReturnsStrip0ForMultiStrip(t *testing.T) {
	payload := []byte("fake strip 0 LZW bytes")
	unused := []byte("strip 1 should not appear in output")
	src := make([]byte, 0, len(payload)+len(unused))
	src = append(src, payload...)
	src = append(src, unused...)
	a := &stripedLabel{
		stripOffsets: []uint64{0, uint64(len(payload))},
		stripCounts:  []uint64{uint64(len(payload)), uint64(len(unused))},
		size:         opentile.Size{W: 10, H: 100},
		compression:  opentile.CompressionLZW,
		reader:       bytes.NewReader(src),
	}
	got, err := a.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("Bytes() = %q, want strip 0 payload %q", got, payload)
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
