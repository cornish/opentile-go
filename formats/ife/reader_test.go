package ife

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestReadUint40LE(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []byte
		want uint64
	}{
		{"zero", []byte{0, 0, 0, 0, 0}, 0},
		{"one", []byte{1, 0, 0, 0, 0}, 1},
		{"all-ones-40bit", []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, NullTile},
		{"max-byte4", []byte{0, 0, 0, 0, 0xFF}, 0xFF00000000},
		{"mixed", []byte{0x78, 0x56, 0x34, 0x12, 0x9A}, 0x9A12345678},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := readUint40LE(tt.in); got != tt.want {
				t.Errorf("readUint40LE(%v) = 0x%x, want 0x%x", tt.in, got, tt.want)
			}
		})
	}
}

func TestReadUint24LE(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   []byte
		want uint32
	}{
		{"zero", []byte{0, 0, 0}, 0},
		{"max-24bit", []byte{0xFF, 0xFF, 0xFF}, 0xFFFFFF},
		{"mixed", []byte{0x34, 0x12, 0xAB}, 0xAB1234},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := readUint24LE(tt.in); got != tt.want {
				t.Errorf("readUint24LE(%v) = 0x%x, want 0x%x", tt.in, got, tt.want)
			}
		})
	}
}

// synthFileHeader builds a 38-byte FILE_HEADER for use in the
// table-driven readFileHeader / readTileTable / etc. tests.
func synthFileHeader(magic uint32, fileSize uint64, extMajor uint16, ttOff, mdOff uint64) []byte {
	b := make([]byte, fileHeaderSize)
	binary.LittleEndian.PutUint32(b[0:4], magic)
	binary.LittleEndian.PutUint16(b[4:6], 0xBEEF) // recovery (irrelevant)
	binary.LittleEndian.PutUint64(b[6:14], fileSize)
	binary.LittleEndian.PutUint16(b[14:16], extMajor)
	binary.LittleEndian.PutUint16(b[16:18], 0) // ext minor
	binary.LittleEndian.PutUint32(b[18:22], 0) // file revision
	binary.LittleEndian.PutUint64(b[22:30], ttOff)
	binary.LittleEndian.PutUint64(b[30:38], mdOff)
	return b
}

func TestReadFileHeader(t *testing.T) {
	const goodSize = uint64(1 << 30) // 1 GiB
	const goodTT = uint64(1024)
	const goodMD = uint64(2048)

	t.Run("ok", func(t *testing.T) {
		buf := synthFileHeader(MagicBytes, goodSize, 1, goodTT, goodMD)
		// Pad out to goodSize so the file_size sanity check passes; the
		// reader only reads the first 38 bytes regardless.
		full := make([]byte, goodSize)
		copy(full, buf)
		r := bytes.NewReader(full)
		hdr, err := readFileHeader(r, int64(goodSize))
		if err != nil {
			t.Fatalf("ok: readFileHeader: %v", err)
		}
		if hdr.MagicBytes != MagicBytes {
			t.Errorf("magic: got 0x%x", hdr.MagicBytes)
		}
		if hdr.TileTableOffset != goodTT || hdr.MetadataOffset != goodMD {
			t.Errorf("offsets: tt=0x%x md=0x%x", hdr.TileTableOffset, hdr.MetadataOffset)
		}
	})

	for _, tc := range []struct {
		name        string
		mutate      func(b []byte)
		fileSize    int64
		wantContain string
	}{
		{"bad magic", func(b []byte) {
			binary.LittleEndian.PutUint32(b[0:4], 0xCAFEBABE)
		}, int64(goodSize), "bad magic"},
		{"file_size mismatch", nil, int64(goodSize) + 1, "file_size mismatch"},
		{"unsupported ext_major", func(b []byte) {
			binary.LittleEndian.PutUint16(b[14:16], 2)
		}, int64(goodSize), "unsupported extension_major"},
		{"NULL tile_table_offset", func(b []byte) {
			binary.LittleEndian.PutUint64(b[22:30], 0)
		}, int64(goodSize), "tile_table_offset is NULL"},
		{"tt_off past EOF", func(b []byte) {
			binary.LittleEndian.PutUint64(b[22:30], goodSize-10)
		}, int64(goodSize), "past EOF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := synthFileHeader(MagicBytes, goodSize, 1, goodTT, goodMD)
			if tc.mutate != nil {
				tc.mutate(buf)
			}
			r := bytes.NewReader(append(buf, make([]byte, 4096)...))
			_, err := readFileHeader(r, tc.fileSize)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantContain) {
				t.Errorf("err = %q; want substring %q", err, tc.wantContain)
			}
		})
	}
}

// synthTileTable returns a 44-byte TILE_TABLE block keyed at the given
// "absolute offset" for validation.
func synthTileTable(off uint64, encoding, format uint8, toOff, leOff uint64, x, y uint32) []byte {
	b := make([]byte, tileTableSize)
	binary.LittleEndian.PutUint64(b[0:8], off)
	binary.LittleEndian.PutUint16(b[8:10], 0)
	b[10] = encoding
	b[11] = format
	binary.LittleEndian.PutUint64(b[12:20], NullOffset)
	binary.LittleEndian.PutUint64(b[20:28], toOff)
	binary.LittleEndian.PutUint64(b[28:36], leOff)
	binary.LittleEndian.PutUint32(b[36:40], x)
	binary.LittleEndian.PutUint32(b[40:44], y)
	return b
}

func TestReadTileTable(t *testing.T) {
	const ttOff = uint64(100)
	const fileSize = int64(1 << 20)
	body := synthTileTable(ttOff, 2, 3, 200, 300, 496, 345)
	buf := make([]byte, fileSize)
	copy(buf[ttOff:], body)
	r := bytes.NewReader(buf)

	tt, err := readTileTable(r, ttOff, fileSize)
	if err != nil {
		t.Fatalf("readTileTable ok: %v", err)
	}
	if tt.Encoding != 2 || tt.Format != 3 {
		t.Errorf("enc/fmt: got %d/%d", tt.Encoding, tt.Format)
	}
	if tt.TileOffsetsOffset != 200 || tt.LayerExtentsOffset != 300 {
		t.Errorf("sub-offs: to=%d le=%d", tt.TileOffsetsOffset, tt.LayerExtentsOffset)
	}

	// Encoding outside {1,2,3}.
	bad := synthTileTable(ttOff, 7, 3, 200, 300, 1, 1)
	copy(buf[ttOff:], bad)
	r = bytes.NewReader(buf)
	if _, err := readTileTable(r, ttOff, fileSize); err == nil ||
		!strings.Contains(err.Error(), "unsupported encoding") {
		t.Errorf("bad encoding: %v", err)
	}

	// Validation mismatch.
	wrong := synthTileTable(0xDEAD, 2, 3, 200, 300, 1, 1)
	copy(buf[ttOff:], wrong)
	r = bytes.NewReader(buf)
	if _, err := readTileTable(r, ttOff, fileSize); err == nil ||
		!strings.Contains(err.Error(), "validation") {
		t.Errorf("validation: %v", err)
	}
}

// synthLayerExtents writes an entire LAYER_EXTENTS block (16-byte hdr +
// N entries) to a buffer at the given offset.
func synthLayerExtents(off uint64, layers []LayerExtent) []byte {
	out := make([]byte, blockHeaderValidation+len(layers)*layerExtentEntrySize)
	binary.LittleEndian.PutUint64(out[0:8], off)
	binary.LittleEndian.PutUint16(out[10:12], layerExtentEntrySize)
	binary.LittleEndian.PutUint32(out[12:16], uint32(len(layers)))
	for i, l := range layers {
		base := blockHeaderValidation + i*layerExtentEntrySize
		binary.LittleEndian.PutUint32(out[base:base+4], l.XTiles)
		binary.LittleEndian.PutUint32(out[base+4:base+8], l.YTiles)
		binary.LittleEndian.PutUint32(out[base+8:base+12], math.Float32bits(l.Scale))
	}
	return out
}

func TestReadLayerExtents(t *testing.T) {
	const off = uint64(50)
	const fileSize = int64(1 << 20)
	good := []LayerExtent{
		{XTiles: 2, YTiles: 2, Scale: 1},
		{XTiles: 4, YTiles: 3, Scale: 2},
		{XTiles: 8, YTiles: 6, Scale: 4},
	}

	buf := make([]byte, fileSize)
	copy(buf[off:], synthLayerExtents(off, good))
	got, err := readLayerExtents(bytes.NewReader(buf), off, fileSize)
	if err != nil {
		t.Fatalf("ok: %v", err)
	}
	if len(got) != len(good) {
		t.Fatalf("len: got %d want %d", len(got), len(good))
	}
	for i := range good {
		if got[i] != good[i] {
			t.Errorf("entry[%d]: got %+v want %+v", i, got[i], good[i])
		}
	}

	// Non-strictly-increasing scales (should reject).
	notIncreasing := []LayerExtent{
		{XTiles: 2, YTiles: 2, Scale: 4},
		{XTiles: 4, YTiles: 3, Scale: 2}, // decreased
	}
	buf2 := make([]byte, fileSize)
	copy(buf2[off:], synthLayerExtents(off, notIncreasing))
	if _, err := readLayerExtents(bytes.NewReader(buf2), off, fileSize); err == nil ||
		!strings.Contains(err.Error(), "strictly increasing") {
		t.Errorf("decreasing-scales: got err = %v", err)
	}

	// Zero entry_number.
	emptyHdr := make([]byte, blockHeaderValidation)
	binary.LittleEndian.PutUint64(emptyHdr[0:8], off)
	binary.LittleEndian.PutUint16(emptyHdr[10:12], layerExtentEntrySize)
	binary.LittleEndian.PutUint32(emptyHdr[12:16], 0)
	buf3 := make([]byte, fileSize)
	copy(buf3[off:], emptyHdr)
	if _, err := readLayerExtents(bytes.NewReader(buf3), off, fileSize); err == nil ||
		!strings.Contains(err.Error(), "entry_number is 0") {
		t.Errorf("empty: got err = %v", err)
	}
}

func TestReadTileOffsets(t *testing.T) {
	const off = uint64(80)
	const fileSize = int64(1 << 20)
	entries := []TileEntry{
		{Offset: 0x1234, Size: 256},
		{Offset: 0xABCDEF, Size: 1024},
		{Offset: NullTile, Size: 0}, // sparse
	}
	body := make([]byte, blockHeaderValidation+len(entries)*tileEntrySize)
	binary.LittleEndian.PutUint64(body[0:8], off)
	binary.LittleEndian.PutUint16(body[10:12], tileEntrySize)
	binary.LittleEndian.PutUint32(body[12:16], uint32(len(entries)))
	for i, e := range entries {
		base := blockHeaderValidation + i*tileEntrySize
		// 40-bit LE offset
		body[base+0] = byte(e.Offset)
		body[base+1] = byte(e.Offset >> 8)
		body[base+2] = byte(e.Offset >> 16)
		body[base+3] = byte(e.Offset >> 24)
		body[base+4] = byte(e.Offset >> 32)
		// 24-bit LE size
		body[base+5] = byte(e.Size)
		body[base+6] = byte(e.Size >> 8)
		body[base+7] = byte(e.Size >> 16)
	}
	buf := make([]byte, fileSize)
	copy(buf[off:], body)

	got, err := readTileOffsets(bytes.NewReader(buf), off, uint64(len(entries)), fileSize)
	if err != nil {
		t.Fatalf("ok: %v", err)
	}
	for i := range entries {
		if got[i] != entries[i] {
			t.Errorf("entry[%d]: got %+v want %+v", i, got[i], entries[i])
		}
	}

	// Mismatched expected count.
	if _, err := readTileOffsets(bytes.NewReader(buf), off, 99, fileSize); err == nil ||
		!strings.Contains(err.Error(), "sum-of-tile-counts") {
		t.Errorf("count mismatch: got err = %v", err)
	}
}
