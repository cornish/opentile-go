//go:build gates

package gates

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestT3LayerOrdering parses TILE_TABLE → LAYER_EXTENTS for cervix
// and confirms upstream's claim: layers stored COARSEST FIRST
// (extents[0].scale < extents[N-1].scale). This is the highest-risk
// gate — if layers are native-first instead, the design's §6
// inversion logic has to flip.
//
// Spec: sample_files/ife/ife-format-spec-for-opentile-go.md
// §"LAYER_EXTENTS" (line 105) and §"Layer ordering: COARSEST
// FIRST" (line 122).
func TestT3LayerOrdering(t *testing.T) {
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

	// 1. FILE_HEADER → tile_table_offset (offset 22, 8 bytes).
	hdr := make([]byte, 38)
	if _, err := f.ReadAt(hdr, 0); err != nil {
		t.Fatalf("ReadAt FILE_HEADER: %v", err)
	}
	tileTableOffset := binary.LittleEndian.Uint64(hdr[22:30])

	// 2. TILE_TABLE (44 bytes) → layer_extents_offset (offset 28),
	//    encoding (offset 10), x_extent/y_extent (offsets 36, 40).
	tt := make([]byte, 44)
	if _, err := f.ReadAt(tt, int64(tileTableOffset)); err != nil {
		t.Fatalf("ReadAt TILE_TABLE: %v", err)
	}
	ttValidation := binary.LittleEndian.Uint64(tt[0:8])
	encoding := tt[10]
	pixFormat := tt[11]
	cipherOffset := binary.LittleEndian.Uint64(tt[12:20])
	tileOffsetsOffset := binary.LittleEndian.Uint64(tt[20:28])
	layerExtentsOffset := binary.LittleEndian.Uint64(tt[28:36])
	xExtent := binary.LittleEndian.Uint32(tt[36:40])
	yExtent := binary.LittleEndian.Uint32(tt[40:44])

	t.Logf("TILE_TABLE @ 0x%x:", tileTableOffset)
	t.Logf("  validation           = 0x%016x (want 0x%016x)", ttValidation, tileTableOffset)
	t.Logf("  encoding             = %d (1=IRIS, 2=JPEG, 3=AVIF)", encoding)
	t.Logf("  format               = %d (3=B8G8R8A8 etc.)", pixFormat)
	t.Logf("  cipher_offset        = 0x%016x (NULL=0xFFFFFFFFFFFFFFFF)", cipherOffset)
	t.Logf("  tile_offsets_offset  = 0x%016x", tileOffsetsOffset)
	t.Logf("  layer_extents_offset = 0x%016x", layerExtentsOffset)
	t.Logf("  x_extent (px)        = %d", xExtent)
	t.Logf("  y_extent (px)        = %d", yExtent)

	if ttValidation != tileTableOffset {
		t.Errorf("TILE_TABLE validation: got 0x%x, want 0x%x", ttValidation, tileTableOffset)
	}
	if encoding != 1 && encoding != 2 && encoding != 3 {
		t.Errorf("encoding: got %d, want one of {1,2,3}", encoding)
	}
	// cervix is JPEG-encoded per the filename.
	if encoding != 2 {
		t.Errorf("cervix encoding: got %d, want 2 (JPEG)", encoding)
	}

	// 3. LAYER_EXTENTS header (16 bytes).
	leHdr := make([]byte, 16)
	if _, err := f.ReadAt(leHdr, int64(layerExtentsOffset)); err != nil {
		t.Fatalf("ReadAt LAYER_EXTENTS header: %v", err)
	}
	leValidation := binary.LittleEndian.Uint64(leHdr[0:8])
	entrySize := binary.LittleEndian.Uint16(leHdr[10:12])
	entryNumber := binary.LittleEndian.Uint32(leHdr[12:16])
	t.Logf("LAYER_EXTENTS @ 0x%x:", layerExtentsOffset)
	t.Logf("  validation   = 0x%016x (want 0x%016x)", leValidation, layerExtentsOffset)
	t.Logf("  entry_size   = %d (want 12)", entrySize)
	t.Logf("  entry_number = %d", entryNumber)

	if leValidation != layerExtentsOffset {
		t.Errorf("LAYER_EXTENTS validation: got 0x%x, want 0x%x", leValidation, layerExtentsOffset)
	}
	if entrySize != 12 {
		t.Errorf("entry_size: got %d, want 12", entrySize)
	}

	// 4. LAYER_EXTENTS entries (entry_number × 12 bytes).
	entriesBuf := make([]byte, int(entryNumber)*int(entrySize))
	if _, err := f.ReadAt(entriesBuf, int64(layerExtentsOffset)+16); err != nil {
		t.Fatalf("ReadAt LAYER_EXTENTS entries: %v", err)
	}

	t.Log("layers (file storage order):")
	scales := make([]float32, entryNumber)
	xTilesArr := make([]uint32, entryNumber)
	yTilesArr := make([]uint32, entryNumber)
	for i := uint32(0); i < entryNumber; i++ {
		base := i * uint32(entrySize)
		xt := binary.LittleEndian.Uint32(entriesBuf[base : base+4])
		yt := binary.LittleEndian.Uint32(entriesBuf[base+4 : base+8])
		sc := math.Float32frombits(binary.LittleEndian.Uint32(entriesBuf[base+8 : base+12]))
		scales[i] = sc
		xTilesArr[i] = xt
		yTilesArr[i] = yt
		t.Logf("  layer %d: x_tiles=%d y_tiles=%d scale=%g (px ≈ %d × %d)",
			i, xt, yt, sc, xt*256, yt*256)
	}

	// 5. Confirm coarsest-first ordering: scales[0] < scales[N-1].
	if entryNumber < 2 {
		t.Skip("only one layer; can't verify ordering")
	}
	first := scales[0]
	last := scales[entryNumber-1]
	t.Logf("scale[0]=%g, scale[N-1]=%g — coarsest-first means scale[0] < scale[N-1]", first, last)
	if first >= last {
		t.Errorf("LAYER ORDERING SURPRISE: scale[0]=%g not less than scale[N-1]=%g; storage is NOT coarsest-first as spec claims — design §6 inversion logic must be re-thought",
			first, last)
	}
	// Strictly increasing across all entries.
	for i := uint32(1); i < entryNumber; i++ {
		if scales[i] <= scales[i-1] {
			t.Errorf("scales not strictly increasing at i=%d: %g vs %g", i, scales[i-1], scales[i])
		}
	}
	// Native (last) scale should produce the file's x_extent/y_extent.
	nativeXPx := xTilesArr[entryNumber-1] * 256
	nativeYPx := yTilesArr[entryNumber-1] * 256
	t.Logf("native layer dims (tile_grid × 256): %d × %d; TILE_TABLE x_extent×y_extent: %d × %d",
		nativeXPx, nativeYPx, xExtent, yExtent)
	if nativeXPx < xExtent || nativeYPx < yExtent {
		t.Errorf("native tile grid (%d×%d px) smaller than TILE_TABLE extent (%d×%d px)",
			nativeXPx, nativeYPx, xExtent, yExtent)
	}
}
