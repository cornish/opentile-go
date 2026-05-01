//go:build gates

package gates

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestT2FileHeader parses the 38-byte FILE_HEADER of cervix_2x_jpeg.iris
// and confirms upstream's structure-offsets claim.
//
// Spec: sample_files/ife/ife-format-spec-for-opentile-go.md
// §"FILE_HEADER" (line 67).
//
// Field layout (all LE):
//
//	magic_bytes       offset  0  uint32   == 0x49726973
//	recovery          offset  4  uint16   (skip)
//	file_size         offset  6  uint64   total file size
//	extension_major   offset 14  uint16   spec major (1)
//	extension_minor   offset 16  uint16   spec minor (0)
//	file_revision     offset 18  uint32   internal revision counter
//	tile_table_offset offset 22  uint64   REQUIRED, never NULL
//	metadata_offset   offset 30  uint64   REQUIRED for spec, reader may ignore
func TestT2FileHeader(t *testing.T) {
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

	stat, err := f.Stat()
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	actualFileSize := stat.Size()

	buf := make([]byte, 38)
	if _, err := f.ReadAt(buf, 0); err != nil {
		t.Fatalf("ReadAt(0, 38): %v", err)
	}
	t.Logf("FILE_HEADER (38 bytes hex):\n  %s", hex.EncodeToString(buf))

	magic := binary.LittleEndian.Uint32(buf[0:4])
	recovery := binary.LittleEndian.Uint16(buf[4:6])
	fileSize := binary.LittleEndian.Uint64(buf[6:14])
	extMajor := binary.LittleEndian.Uint16(buf[14:16])
	extMinor := binary.LittleEndian.Uint16(buf[16:18])
	fileRev := binary.LittleEndian.Uint32(buf[18:22])
	tileTableOffset := binary.LittleEndian.Uint64(buf[22:30])
	metadataOffset := binary.LittleEndian.Uint64(buf[30:38])

	t.Logf("  magic_bytes       = 0x%08x", magic)
	t.Logf("  recovery          = 0x%04x", recovery)
	t.Logf("  file_size         = %d (actual: %d)", fileSize, actualFileSize)
	t.Logf("  extension_major   = %d", extMajor)
	t.Logf("  extension_minor   = %d", extMinor)
	t.Logf("  file_revision     = %d", fileRev)
	t.Logf("  tile_table_offset = 0x%016x (%d)", tileTableOffset, tileTableOffset)
	t.Logf("  metadata_offset   = 0x%016x (%d)", metadataOffset, metadataOffset)

	// Pin the asserts.
	if magic != 0x49726973 {
		t.Errorf("magic: got 0x%08x, want 0x49726973", magic)
	}
	if int64(fileSize) != actualFileSize {
		t.Errorf("file_size: header says %d, stat says %d", fileSize, actualFileSize)
	}
	if extMajor != 1 {
		t.Errorf("extension_major: got %d, want 1 (v1.0 only)", extMajor)
	}
	if tileTableOffset == 0 {
		t.Error("tile_table_offset is zero — must be non-NULL per spec")
	}
	if tileTableOffset >= uint64(actualFileSize) {
		t.Errorf("tile_table_offset 0x%x past EOF (file size %d)", tileTableOffset, actualFileSize)
	}
	if metadataOffset >= uint64(actualFileSize) {
		t.Errorf("metadata_offset 0x%x past EOF", metadataOffset)
	}
}
