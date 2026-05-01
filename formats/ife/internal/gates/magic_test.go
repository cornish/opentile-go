//go:build gates

// Package gates holds the v0.8 JIT verification probes that run
// before any production code lands. Build-tag `gates` keeps them
// out of `make test`. Deleted at end of v0.8 milestone.
package gates

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestT1Magic probes the first 16 bytes of cervix_2x_jpeg.iris and
// confirms upstream's magic-bytes claim: MAGIC_BYTES = 0x49726973
// ("Iris" as LE uint32) at file offset 0–3, always little-endian.
//
// Spec: sample_files/ife/ife-format-spec-for-opentile-go.md §"Magic
// bytes" (line 52) and §"Endianness" (line 21).
func TestT1Magic(t *testing.T) {
	dir := os.Getenv("OPENTILE_TESTDIR")
	if dir == "" {
		t.Skip("OPENTILE_TESTDIR not set")
	}
	path := filepath.Join(dir, "ife", "cervix_2x_jpeg.iris")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	buf := make([]byte, 16)
	if _, err := f.ReadAt(buf, 0); err != nil {
		t.Fatalf("ReadAt(0, 16): %v", err)
	}
	t.Logf("first 16 bytes (hex): %s", hex.EncodeToString(buf))
	t.Logf("first 16 bytes (raw): %q", buf)

	const wantMagic uint32 = 0x49726973
	gotMagic := binary.LittleEndian.Uint32(buf[0:4])
	t.Logf("magic LE uint32: 0x%08x (want 0x%08x)", gotMagic, wantMagic)
	if gotMagic != wantMagic {
		t.Errorf("magic mismatch: got 0x%08x, want 0x%08x", gotMagic, wantMagic)
	}

	// Sanity: bytes 0..3 should be exactly 0x73 0x69 0x72 0x49 on
	// disk (little-endian assembly of "Iris" == 0x49 0x72 0x69 0x73).
	wantBytes := []byte{0x73, 0x69, 0x72, 0x49}
	for i, b := range wantBytes {
		if buf[i] != b {
			t.Errorf("byte[%d]: got 0x%02x, want 0x%02x", i, buf[i], b)
		}
	}
}
