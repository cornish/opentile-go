//go:build gates

package gates

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// TestT4TileOffsets parses TILE_OFFSETS for cervix and confirms:
//
//  1. validation field == tile_offsets_offset.
//  2. entry_size == 8, entry_number == sum(x_tiles * y_tiles) over
//     all layers.
//  3. The 40-bit offset + 24-bit size encoding (5 + 3 bytes per entry)
//     parses cleanly. Offsets land within [0, file_size).
//  4. NULL_TILE sentinel = 0xFFFFFFFFFF (40-bit all-1s) — count any
//     occurrences in the first ~100 entries; cervix is fully-tiled
//     so we expect zero, but the probe must not crash on absence.
//
// Spec: sample_files/ife/ife-format-spec-for-opentile-go.md §"TILE_OFFSETS"
// (line 132) and §"sparse / empty tile" (line 153).
func TestT4TileOffsets(t *testing.T) {
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
	stat, _ := f.Stat()
	fileSize := uint64(stat.Size())

	// Walk FILE_HEADER → TILE_TABLE to find tile_offsets_offset and
	// to compute expected entry_number from LAYER_EXTENTS.
	hdr := make([]byte, 38)
	if _, err := f.ReadAt(hdr, 0); err != nil {
		t.Fatalf("ReadAt FILE_HEADER: %v", err)
	}
	tileTableOffset := binary.LittleEndian.Uint64(hdr[22:30])

	tt := make([]byte, 44)
	if _, err := f.ReadAt(tt, int64(tileTableOffset)); err != nil {
		t.Fatalf("ReadAt TILE_TABLE: %v", err)
	}
	tileOffsetsOffset := binary.LittleEndian.Uint64(tt[20:28])
	layerExtentsOffset := binary.LittleEndian.Uint64(tt[28:36])

	// LAYER_EXTENTS → expected total tile count.
	leHdr := make([]byte, 16)
	if _, err := f.ReadAt(leHdr, int64(layerExtentsOffset)); err != nil {
		t.Fatalf("ReadAt LAYER_EXTENTS hdr: %v", err)
	}
	leEntryNumber := binary.LittleEndian.Uint32(leHdr[12:16])
	entriesBuf := make([]byte, int(leEntryNumber)*12)
	if _, err := f.ReadAt(entriesBuf, int64(layerExtentsOffset)+16); err != nil {
		t.Fatalf("ReadAt LAYER_EXTENTS entries: %v", err)
	}
	var expectedTotal uint64
	for i := uint32(0); i < leEntryNumber; i++ {
		base := i * 12
		xt := binary.LittleEndian.Uint32(entriesBuf[base : base+4])
		yt := binary.LittleEndian.Uint32(entriesBuf[base+4 : base+8])
		expectedTotal += uint64(xt) * uint64(yt)
	}
	t.Logf("expected total tiles (sum across layers): %d", expectedTotal)

	// TILE_OFFSETS header (16 bytes).
	toHdr := make([]byte, 16)
	if _, err := f.ReadAt(toHdr, int64(tileOffsetsOffset)); err != nil {
		t.Fatalf("ReadAt TILE_OFFSETS hdr: %v", err)
	}
	toValidation := binary.LittleEndian.Uint64(toHdr[0:8])
	toEntrySize := binary.LittleEndian.Uint16(toHdr[10:12])
	toEntryNumber := binary.LittleEndian.Uint32(toHdr[12:16])

	t.Logf("TILE_OFFSETS @ 0x%x:", tileOffsetsOffset)
	t.Logf("  validation   = 0x%016x (want 0x%016x)", toValidation, tileOffsetsOffset)
	t.Logf("  entry_size   = %d (want 8)", toEntrySize)
	t.Logf("  entry_number = %d (want %d)", toEntryNumber, expectedTotal)

	if toValidation != tileOffsetsOffset {
		t.Errorf("TILE_OFFSETS validation: got 0x%x, want 0x%x", toValidation, tileOffsetsOffset)
	}
	if toEntrySize != 8 {
		t.Errorf("entry_size: got %d, want 8", toEntrySize)
	}
	if uint64(toEntryNumber) != expectedTotal {
		t.Errorf("entry_number: got %d, want %d (sum of layer x_tiles*y_tiles)",
			toEntryNumber, expectedTotal)
	}

	// Read first 100 entries (or all if fewer) and validate offsets.
	const sampleN = 100
	n := uint32(sampleN)
	if toEntryNumber < n {
		n = toEntryNumber
	}
	sampleBuf := make([]byte, int(n)*8)
	if _, err := f.ReadAt(sampleBuf, int64(tileOffsetsOffset)+16); err != nil {
		t.Fatalf("ReadAt sample entries: %v", err)
	}

	const nullTile uint64 = 0xFFFFFFFFFF // 40-bit all-1s
	sparseCount := 0
	var minOff, maxOff uint64 = ^uint64(0), 0
	var minSize, maxSize uint32 = ^uint32(0), 0
	outOfRange := 0

	for i := uint32(0); i < n; i++ {
		base := i * 8
		entry := sampleBuf[base : base+8]
		// 40-bit offset (LE) = bytes 0..4 with byte 4 as the high byte.
		off := uint64(entry[0]) |
			uint64(entry[1])<<8 |
			uint64(entry[2])<<16 |
			uint64(entry[3])<<24 |
			uint64(entry[4])<<32
		// 24-bit size (LE) = bytes 5..7.
		size := uint32(entry[5]) |
			uint32(entry[6])<<8 |
			uint32(entry[7])<<16

		if off == nullTile {
			sparseCount++
			continue
		}
		if off >= fileSize {
			outOfRange++
		}
		if off < minOff {
			minOff = off
		}
		if off > maxOff {
			maxOff = off
		}
		if size < minSize {
			minSize = size
		}
		if size > maxSize {
			maxSize = size
		}
	}

	t.Logf("sample of first %d TILE_OFFSETS entries:", n)
	t.Logf("  sparse (NULL_TILE = 0xFFFFFFFFFF) count: %d", sparseCount)
	t.Logf("  non-sparse offset range: [0x%x, 0x%x]", minOff, maxOff)
	t.Logf("  non-sparse size    range: [%d, %d] bytes", minSize, maxSize)
	if outOfRange > 0 {
		t.Errorf("%d sample entries have offset >= file_size (%d)", outOfRange, fileSize)
	}
	if maxOff >= tileOffsetsOffset {
		t.Errorf("non-sparse offset 0x%x lands inside or past TILE_OFFSETS block (0x%x)",
			maxOff, tileOffsetsOffset)
	}
}
