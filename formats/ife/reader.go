// Package ife implements opentile-go format support for Iris File Extension
// (IFE) v1.0 — the IrisDigitalPathology bleeding-edge non-TIFF WSI container.
// Tiles are returned as raw compressed bytes (JPEG, AVIF, or the
// Iris-proprietary codec) without decoding.
//
// Spec: sample_files/ife/ife-format-spec-for-opentile-go.md.
// Upstream: https://github.com/IrisDigitalPathology/Iris-File-Extension
// (MIT) and https://github.com/IrisDigitalPathology/Iris-Headers.
package ife

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// MagicBytes is "Iris" assembled as a little-endian uint32 at file
// offset 0..3. On disk the bytes appear in order 0x73 0x69 0x72 0x49.
const MagicBytes uint32 = 0x49726973

// NullOffset is the placeholder value used for absent block pointers in
// FILE_HEADER and TILE_TABLE (e.g. cipher_offset on files without
// encryption). 8-byte all-1s.
const NullOffset uint64 = 0xFFFFFFFFFFFFFFFF

// NullTile marks a sparse / empty entry in TILE_OFFSETS — a tile-grid
// cell with no compressed bytes on disk. 5-byte all-1s (the offset
// portion of the 5+3 entry encoding).
const NullTile uint64 = 0xFFFFFFFFFF

// TileSidePixels is the fixed tile dimension in pixels at every layer.
// IFE v1.0 hard-codes this; not configurable.
const TileSidePixels = 256

// FileHeader is the 38-byte fixed header at file offset 0 of every IFE
// v1.0 file.
type FileHeader struct {
	MagicBytes       uint32
	Recovery         uint16
	FileSize         uint64
	ExtensionMajor   uint16
	ExtensionMinor   uint16
	FileRevision     uint32
	TileTableOffset  uint64
	MetadataOffset   uint64
}

const fileHeaderSize = 38

// TileTable is the 44-byte block at FileHeader.TileTableOffset that
// pivots between LAYER_EXTENTS and TILE_OFFSETS plus carries per-file
// codec / pixel-format identifiers.
type TileTable struct {
	Validation         uint64
	Recovery           uint16
	Encoding           uint8 // 0=undef, 1=IRIS, 2=JPEG, 3=AVIF
	Format             uint8 // 0=undef, 1=B8G8R8, 2=R8G8B8, 3=B8G8R8A8, 4=R8G8B8A8
	CipherOffset       uint64
	TileOffsetsOffset  uint64
	LayerExtentsOffset uint64
	XExtent            uint32 // see note in Open: cervix carries tile counts here, not pixels
	YExtent            uint32
}

const tileTableSize = 44

// LayerExtent is one entry from the LAYER_EXTENTS array. Layers are
// stored coarsest-first in the file; the API-facing slice is reversed
// at parse time so Levels()[0] is native.
type LayerExtent struct {
	XTiles uint32  // tile-grid width at this layer
	YTiles uint32  // tile-grid height at this layer
	Scale  float32 // downsampling factor; higher value = finer resolution
}

const layerExtentEntrySize = 12

// TileEntry is one entry from the TILE_OFFSETS array — a (40-bit
// offset, 24-bit size) pair. Sparse entries carry Offset == NullTile.
type TileEntry struct {
	Offset uint64 // 40-bit byte offset to compressed tile data; max 1 TB
	Size   uint32 // 24-bit length of compressed data; max 16 MB
}

const tileEntrySize = 8

// blockHeaderValidation is the size of the leading "validation +
// recovery + entry_size + entry_number" prefix on TILE_OFFSETS and
// LAYER_EXTENTS (16 bytes total).
const blockHeaderValidation = 16

// readUint40LE reads a 40-bit little-endian unsigned integer from b. b
// must have at least 5 bytes; only the first five are consumed.
func readUint40LE(b []byte) uint64 {
	_ = b[4]
	return uint64(b[0]) |
		uint64(b[1])<<8 |
		uint64(b[2])<<16 |
		uint64(b[3])<<24 |
		uint64(b[4])<<32
}

// readUint24LE reads a 24-bit little-endian unsigned integer from b. b
// must have at least 3 bytes; only the first three are consumed.
func readUint24LE(b []byte) uint32 {
	_ = b[2]
	return uint32(b[0]) |
		uint32(b[1])<<8 |
		uint32(b[2])<<16
}

// readFileHeader parses the 38-byte FILE_HEADER from r at offset 0.
// Fails if MagicBytes mismatch, file_size doesn't match the supplied
// fileSize, ExtensionMajor != 1, or required offsets are NULL.
func readFileHeader(r io.ReaderAt, fileSize int64) (FileHeader, error) {
	var hdr FileHeader
	buf := make([]byte, fileHeaderSize)
	if _, err := r.ReadAt(buf, 0); err != nil {
		return hdr, fmt.Errorf("ife: read FILE_HEADER: %w", err)
	}

	hdr.MagicBytes = binary.LittleEndian.Uint32(buf[0:4])
	if hdr.MagicBytes != MagicBytes {
		return hdr, fmt.Errorf("ife: bad magic 0x%08x; want 0x%08x", hdr.MagicBytes, MagicBytes)
	}
	hdr.Recovery = binary.LittleEndian.Uint16(buf[4:6])
	hdr.FileSize = binary.LittleEndian.Uint64(buf[6:14])
	if int64(hdr.FileSize) != fileSize {
		return hdr, fmt.Errorf("ife: file_size mismatch: header says %d, supplied %d",
			hdr.FileSize, fileSize)
	}
	hdr.ExtensionMajor = binary.LittleEndian.Uint16(buf[14:16])
	if hdr.ExtensionMajor != 1 {
		return hdr, fmt.Errorf("ife: unsupported extension_major %d (only v1 supported)",
			hdr.ExtensionMajor)
	}
	hdr.ExtensionMinor = binary.LittleEndian.Uint16(buf[16:18])
	hdr.FileRevision = binary.LittleEndian.Uint32(buf[18:22])
	hdr.TileTableOffset = binary.LittleEndian.Uint64(buf[22:30])
	hdr.MetadataOffset = binary.LittleEndian.Uint64(buf[30:38])

	if hdr.TileTableOffset == 0 || hdr.TileTableOffset == NullOffset {
		return hdr, errors.New("ife: tile_table_offset is NULL but spec requires it")
	}
	if int64(hdr.TileTableOffset)+tileTableSize > fileSize {
		return hdr, fmt.Errorf("ife: tile_table_offset 0x%x past EOF (file %d bytes)",
			hdr.TileTableOffset, fileSize)
	}
	return hdr, nil
}

// readTileTable parses the 44-byte TILE_TABLE block at off. Fails if
// the validation field doesn't echo off, the encoding is undefined or
// outside the {1,2,3} set, or required sub-offsets are NULL / past EOF.
func readTileTable(r io.ReaderAt, off uint64, fileSize int64) (TileTable, error) {
	var tt TileTable
	if int64(off)+tileTableSize > fileSize {
		return tt, fmt.Errorf("ife: TILE_TABLE off 0x%x past EOF", off)
	}
	buf := make([]byte, tileTableSize)
	if _, err := r.ReadAt(buf, int64(off)); err != nil {
		return tt, fmt.Errorf("ife: read TILE_TABLE: %w", err)
	}

	tt.Validation = binary.LittleEndian.Uint64(buf[0:8])
	if tt.Validation != off {
		return tt, fmt.Errorf("ife: TILE_TABLE validation 0x%x != offset 0x%x",
			tt.Validation, off)
	}
	tt.Recovery = binary.LittleEndian.Uint16(buf[8:10])
	tt.Encoding = buf[10]
	tt.Format = buf[11]
	tt.CipherOffset = binary.LittleEndian.Uint64(buf[12:20])
	tt.TileOffsetsOffset = binary.LittleEndian.Uint64(buf[20:28])
	tt.LayerExtentsOffset = binary.LittleEndian.Uint64(buf[28:36])
	tt.XExtent = binary.LittleEndian.Uint32(buf[36:40])
	tt.YExtent = binary.LittleEndian.Uint32(buf[40:44])

	if tt.Encoding == 0 || tt.Encoding > 3 {
		return tt, fmt.Errorf("ife: unsupported encoding %d (want 1=IRIS, 2=JPEG, 3=AVIF)",
			tt.Encoding)
	}
	if tt.TileOffsetsOffset == 0 || tt.TileOffsetsOffset == NullOffset {
		return tt, errors.New("ife: tile_offsets_offset is NULL")
	}
	if tt.LayerExtentsOffset == 0 || tt.LayerExtentsOffset == NullOffset {
		return tt, errors.New("ife: layer_extents_offset is NULL")
	}
	if int64(tt.TileOffsetsOffset) >= fileSize {
		return tt, fmt.Errorf("ife: tile_offsets_offset 0x%x past EOF", tt.TileOffsetsOffset)
	}
	if int64(tt.LayerExtentsOffset) >= fileSize {
		return tt, fmt.Errorf("ife: layer_extents_offset 0x%x past EOF", tt.LayerExtentsOffset)
	}
	return tt, nil
}

// readLayerExtents parses the LAYER_EXTENTS block at off and returns
// the entries in their on-disk order (coarsest-first). Caller is
// responsible for inverting if the API needs native-first.
func readLayerExtents(r io.ReaderAt, off uint64, fileSize int64) ([]LayerExtent, error) {
	if int64(off)+blockHeaderValidation > fileSize {
		return nil, fmt.Errorf("ife: LAYER_EXTENTS hdr off 0x%x past EOF", off)
	}
	hdr := make([]byte, blockHeaderValidation)
	if _, err := r.ReadAt(hdr, int64(off)); err != nil {
		return nil, fmt.Errorf("ife: read LAYER_EXTENTS hdr: %w", err)
	}
	validation := binary.LittleEndian.Uint64(hdr[0:8])
	if validation != off {
		return nil, fmt.Errorf("ife: LAYER_EXTENTS validation 0x%x != offset 0x%x",
			validation, off)
	}
	entrySize := binary.LittleEndian.Uint16(hdr[10:12])
	if entrySize != layerExtentEntrySize {
		return nil, fmt.Errorf("ife: LAYER_EXTENTS entry_size %d, want %d",
			entrySize, layerExtentEntrySize)
	}
	entryNumber := binary.LittleEndian.Uint32(hdr[12:16])
	if entryNumber == 0 {
		return nil, errors.New("ife: LAYER_EXTENTS entry_number is 0; need at least 1 layer")
	}

	totalSize := int64(entryNumber) * int64(entrySize)
	if int64(off)+blockHeaderValidation+totalSize > fileSize {
		return nil, fmt.Errorf("ife: LAYER_EXTENTS body would extend past EOF")
	}
	body := make([]byte, totalSize)
	if _, err := r.ReadAt(body, int64(off)+blockHeaderValidation); err != nil {
		return nil, fmt.Errorf("ife: read LAYER_EXTENTS body: %w", err)
	}

	out := make([]LayerExtent, entryNumber)
	for i := uint32(0); i < entryNumber; i++ {
		base := i * uint32(entrySize)
		out[i].XTiles = binary.LittleEndian.Uint32(body[base : base+4])
		out[i].YTiles = binary.LittleEndian.Uint32(body[base+4 : base+8])
		out[i].Scale = math.Float32frombits(binary.LittleEndian.Uint32(body[base+8 : base+12]))
		if out[i].XTiles == 0 || out[i].YTiles == 0 {
			return nil, fmt.Errorf("ife: layer %d has zero-dim tile grid (%d × %d)",
				i, out[i].XTiles, out[i].YTiles)
		}
	}
	// Pin coarsest-first: scales must be strictly increasing.
	for i := uint32(1); i < entryNumber; i++ {
		if !(out[i].Scale > out[i-1].Scale) {
			return nil, fmt.Errorf("ife: LAYER_EXTENTS scales not strictly increasing at i=%d (%g vs %g); spec requires coarsest-first storage",
				i, out[i-1].Scale, out[i].Scale)
		}
	}
	return out, nil
}

// readTileOffsets parses the TILE_OFFSETS block at off and returns
// entryNumber entries (5+3 byte encoding). expected is the
// sum-of-tile-counts across all layers from LAYER_EXTENTS; the parser
// errors if the file's entry_number doesn't match.
func readTileOffsets(r io.ReaderAt, off uint64, expected uint64, fileSize int64) ([]TileEntry, error) {
	if int64(off)+blockHeaderValidation > fileSize {
		return nil, fmt.Errorf("ife: TILE_OFFSETS hdr off 0x%x past EOF", off)
	}
	hdr := make([]byte, blockHeaderValidation)
	if _, err := r.ReadAt(hdr, int64(off)); err != nil {
		return nil, fmt.Errorf("ife: read TILE_OFFSETS hdr: %w", err)
	}
	validation := binary.LittleEndian.Uint64(hdr[0:8])
	if validation != off {
		return nil, fmt.Errorf("ife: TILE_OFFSETS validation 0x%x != offset 0x%x",
			validation, off)
	}
	entrySize := binary.LittleEndian.Uint16(hdr[10:12])
	if entrySize != tileEntrySize {
		return nil, fmt.Errorf("ife: TILE_OFFSETS entry_size %d, want %d",
			entrySize, tileEntrySize)
	}
	entryNumber := binary.LittleEndian.Uint32(hdr[12:16])
	if uint64(entryNumber) != expected {
		return nil, fmt.Errorf("ife: TILE_OFFSETS entry_number %d != sum-of-tile-counts %d",
			entryNumber, expected)
	}

	totalSize := int64(entryNumber) * int64(entrySize)
	if int64(off)+blockHeaderValidation+totalSize > fileSize {
		return nil, fmt.Errorf("ife: TILE_OFFSETS body would extend past EOF")
	}
	body := make([]byte, totalSize)
	if _, err := r.ReadAt(body, int64(off)+blockHeaderValidation); err != nil {
		return nil, fmt.Errorf("ife: read TILE_OFFSETS body: %w", err)
	}

	out := make([]TileEntry, entryNumber)
	for i := uint32(0); i < entryNumber; i++ {
		base := i * uint32(entrySize)
		out[i].Offset = readUint40LE(body[base : base+5])
		out[i].Size = readUint24LE(body[base+5 : base+8])
	}
	return out, nil
}
