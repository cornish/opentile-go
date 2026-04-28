package bif

import (
	"bytes"
	"encoding/binary"
	"testing"

	opentile "github.com/cornish/opentile-go"
	"github.com/cornish/opentile-go/internal/tiff"
)

// TestSupportsBIFWithIScan: BigTIFF whose IFD 0 XMP packet contains
// `<iScan` matches.
func TestSupportsBIFWithIScan(t *testing.T) {
	data := buildBIFLikeBigTIFF(t, []iFDSpec{{xmp: []byte(`<iScan ScannerModel="VENTANA DP 200"/>`)}})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if !New().Supports(f) {
		t.Fatal("expected Supports=true on BigTIFF with `<iScan` in XMP")
	}
}

// TestSupportsBIFWithIScanOnLaterIFD: detection must walk every IFD,
// not just IFD 0. Spec-compliant BIF carries `<iScan` in IFD 0 *and*
// IFD 2; legacy iScan carries it in IFD 0 *and* IFD 2 as well. Both
// pages have it for our local fixtures, but the rule is "any IFD" —
// confirm that semantics.
func TestSupportsBIFWithIScanOnLaterIFD(t *testing.T) {
	data := buildBIFLikeBigTIFF(t, []iFDSpec{
		{xmp: []byte(`<somethingelse/>`)},
		{xmp: []byte(`<iScan/>`)},
	})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if !New().Supports(f) {
		t.Fatal("expected Supports=true when `<iScan` appears in a non-first IFD")
	}
}

// TestSupportsRejectsClassicTIFFWithIScan: detection requires BigTIFF
// per spec §5.1; a classic TIFF whose XMP contains `<iScan` must NOT
// match. Classic-TIFF iScan files don't exist (the BIF whitepaper
// mandates BigTIFF) but we double-check the gate.
func TestSupportsRejectsClassicTIFFWithIScan(t *testing.T) {
	data := buildClassicTIFFWithXMP(t, []byte(`<iScan/>`))
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if New().Supports(f) {
		t.Fatal("expected Supports=false on classic TIFF (BigTIFF required)")
	}
}

// TestSupportsRejectsBigTIFFWithoutXMP: BigTIFF without any XMP tag
// must not match.
func TestSupportsRejectsBigTIFFWithoutXMP(t *testing.T) {
	data := buildBIFLikeBigTIFF(t, []iFDSpec{{xmp: nil}})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if New().Supports(f) {
		t.Fatal("expected Supports=false on BigTIFF without XMP")
	}
}

// TestSupportsRejectsBigTIFFWithUnrelatedXMP: BigTIFF with an XMP tag
// whose contents do NOT include `<iScan` (e.g., an OME-style or SVS-
// style XMP packet) must not match.
func TestSupportsRejectsBigTIFFWithUnrelatedXMP(t *testing.T) {
	data := buildBIFLikeBigTIFF(t, []iFDSpec{
		{xmp: []byte(`<DataObject ObjectType="DPUfsImport"/>`)},
	})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if New().Supports(f) {
		t.Fatal("expected Supports=false on BigTIFF with non-iScan XMP")
	}
}

// TestFormatIdentity confirms the FormatBIF constant.
func TestFormatIdentity(t *testing.T) {
	if got := New().Format(); got != opentile.FormatBIF {
		t.Errorf("Format(): got %q, want %q", got, opentile.FormatBIF)
	}
}

// TestOpenStubReturnsErrUnsupportedOnNonBIF: Open must enforce the
// detection gate even though Factory.Supports already does.
func TestOpenStubReturnsErrUnsupportedOnNonBIF(t *testing.T) {
	data := buildBIFLikeBigTIFF(t, []iFDSpec{{xmp: nil}})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	if _, err := New().Open(f, nil); err != opentile.ErrUnsupportedFormat {
		t.Errorf("Open: got %v, want ErrUnsupportedFormat", err)
	}
}

// TestOpenStubReturnsTilerOnBIF: Open returns a non-nil Tiler when
// detection passes and at least one pyramid IFD is present.
// Subsequent T13+ tasks populate Image / Level content.
func TestOpenStubReturnsTilerOnBIF(t *testing.T) {
	data := buildBIFLikeBigTIFF(t, []iFDSpec{
		{xmp: []byte(`<iScan/>`), description: "Label_Image"},
		{description: "level=0 mag=40 quality=95", imageWidth: 256, imageLength: 256, tileWidth: 256, tileLength: 256},
	})
	f, err := tiff.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("tiff.Open: %v", err)
	}
	tiler, err := New().Open(f, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if tiler == nil {
		t.Fatal("Open returned nil Tiler with no error")
	}
	if tiler.Format() != opentile.FormatBIF {
		t.Errorf("Format(): got %q, want %q", tiler.Format(), opentile.FormatBIF)
	}
}

// iFDSpec describes the contents of one IFD in a synthetic BIF-like
// BigTIFF. ImageWidth/ImageLength default to 1024×768 (override via
// imageWidth/imageLength); XMP and ImageDescription are optional.
// When tileWidth > 0, the IFD also carries TileWidth/TileLength/
// TileOffsets/TileByteCounts tags — a complete tile pyramid IFD.
//
// The synthetic builder is shared across detection / classification /
// layout / level tests; downstream tile-reader tests (T14 / T15)
// will extend it with JPEGTables and explicit per-tile content.
type iFDSpec struct {
	xmp         []byte // nil = omit XMP tag (700)
	description string // empty = omit ImageDescription tag (270)

	// imageWidth/imageLength default to 1024 / 768 when zero. They
	// determine the file's stated image dimensions. For pyramid
	// IFDs (tileWidth > 0), the TileGrid is computed as
	// ceil(imageWidth/tileWidth) × ceil(imageLength/tileLength).
	imageWidth, imageLength int

	// Tile metadata. When tileWidth == 0 (default), the IFD is
	// strip-based or otherwise not a pyramid IFD: no Tile* tags.
	// When tileWidth > 0, the IFD becomes a tiled JPEG layout with
	// TileWidth / TileLength tags and TileOffsets / TileByteCounts
	// arrays. Each tile's bytes are filled with `tileFill` (default
	// 0x00) — not real JPEG, but enough to exercise the offset/length
	// plumbing in level.go.
	tileWidth, tileLength int
	tileFill              byte // arbitrary fill byte, default 0
}

// buildBIFLikeBigTIFF builds a BigTIFF (little-endian) carrying len(ifds)
// IFDs. Each IFD carries ImageWidth=1024, ImageLength=768, and (if
// xmp != nil) an XMP tag (700, type 7 UNDEFINED) referencing the
// payload bytes appended after all IFDs.
//
// Layout:
//
//	[0..16)         BigTIFF header (firstIFD = 0x10)
//	[0x10..)        IFD entries, payload areas, next IFD links
//	After everything: XMP payload bytes for every IFD that has them
func buildBIFLikeBigTIFF(t *testing.T, ifds []iFDSpec) []byte {
	t.Helper()
	if len(ifds) == 0 {
		t.Fatal("need at least one IFD")
	}
	buf := new(bytes.Buffer)
	// BigTIFF header: II 0x2B 0x00 offsetSize=8 const=0 firstIFD(8)=0x10
	buf.Write([]byte{'I', 'I', 0x2B, 0x00, 0x08, 0x00, 0x00, 0x00})
	_ = binary.Write(buf, binary.LittleEndian, uint64(0x10))

	// Per-IFD computed metadata. Defaults applied here so the layout
	// pass below treats every entry uniformly.
	type meta struct {
		entryCount uint64
		descBytes  []byte // ASCII bytes for ImageDescription, NUL-terminated; nil if absent
		imgW, imgH uint64 // ImageWidth/ImageLength values
		tw, th     uint64 // TileWidth/TileLength values; 0 = no tile tags
		gridW      uint64
		gridH      uint64
		tileBytes  []byte // concatenated raw tile bytes; nil if no tile tags
	}
	metas := make([]meta, len(ifds))
	for i, ifd := range ifds {
		m := &metas[i]
		m.imgW = uint64(ifd.imageWidth)
		if m.imgW == 0 {
			m.imgW = 1024
		}
		m.imgH = uint64(ifd.imageLength)
		if m.imgH == 0 {
			m.imgH = 768
		}
		m.entryCount = 2 // ImageWidth + ImageLength baseline
		if ifd.xmp != nil {
			m.entryCount++
		}
		if ifd.description != "" {
			m.entryCount++
			m.descBytes = append([]byte(ifd.description), 0)
		}
		if ifd.tileWidth > 0 {
			m.tw = uint64(ifd.tileWidth)
			m.th = uint64(ifd.tileLength)
			m.gridW = (m.imgW + m.tw - 1) / m.tw
			m.gridH = (m.imgH + m.th - 1) / m.th
			n := m.gridW * m.gridH
			tileSize := m.tw * m.th
			m.tileBytes = bytes.Repeat([]byte{ifd.tileFill}, int(n*tileSize))
			m.entryCount += 4 // TileWidth + TileLength + TileOffsets + TileByteCounts
		}
	}

	// Pass 1: compute IFD offsets head-to-toe.
	ifdOffsets := make([]uint64, len(ifds))
	cursor := uint64(0x10)
	for i := range ifds {
		ifdOffsets[i] = cursor
		cursor += 8 + metas[i].entryCount*20 + 8
	}
	// Pass 2: assign out-of-line payload offsets.
	xmpOffsets := make([]uint64, len(ifds))
	descOffsets := make([]uint64, len(ifds))
	tileOffArrayOffsets := make([]uint64, len(ifds))
	tileCntArrayOffsets := make([]uint64, len(ifds))
	tileDataOffsets := make([]uint64, len(ifds))
	for i, ifd := range ifds {
		m := &metas[i]
		if ifd.xmp != nil && len(ifd.xmp) > 8 {
			xmpOffsets[i] = cursor
			cursor += uint64(len(ifd.xmp))
		}
		if len(m.descBytes) > 8 {
			descOffsets[i] = cursor
			cursor += uint64(len(m.descBytes))
		}
		if ifd.tileWidth > 0 {
			n := m.gridW * m.gridH
			// TileOffsets array (LONG8 = 8 bytes/entry). Inline only when n == 1.
			if n > 1 {
				tileOffArrayOffsets[i] = cursor
				cursor += n * 8
				tileCntArrayOffsets[i] = cursor
				cursor += n * 8
			}
			tileDataOffsets[i] = cursor
			cursor += uint64(len(m.tileBytes))
		}
	}

	// Pass 3: emit IFDs.
	for i, ifd := range ifds {
		m := &metas[i]
		_ = binary.Write(buf, binary.LittleEndian, m.entryCount)
		// ImageWidth (256, LONG, count=1)
		_ = binary.Write(buf, binary.LittleEndian, uint16(256))
		_ = binary.Write(buf, binary.LittleEndian, uint16(4))
		_ = binary.Write(buf, binary.LittleEndian, uint64(1))
		_ = binary.Write(buf, binary.LittleEndian, m.imgW)
		// ImageLength (257, LONG, count=1)
		_ = binary.Write(buf, binary.LittleEndian, uint16(257))
		_ = binary.Write(buf, binary.LittleEndian, uint16(4))
		_ = binary.Write(buf, binary.LittleEndian, uint64(1))
		_ = binary.Write(buf, binary.LittleEndian, m.imgH)
		if ifd.description != "" {
			_ = binary.Write(buf, binary.LittleEndian, uint16(270))
			_ = binary.Write(buf, binary.LittleEndian, uint16(2))
			_ = binary.Write(buf, binary.LittleEndian, uint64(len(m.descBytes)))
			writeInlineOrOffset(buf, m.descBytes, descOffsets[i])
		}
		if ifd.xmp != nil {
			_ = binary.Write(buf, binary.LittleEndian, uint16(700))
			_ = binary.Write(buf, binary.LittleEndian, uint16(7))
			_ = binary.Write(buf, binary.LittleEndian, uint64(len(ifd.xmp)))
			writeInlineOrOffset(buf, ifd.xmp, xmpOffsets[i])
		}
		if ifd.tileWidth > 0 {
			n := m.gridW * m.gridH
			tileSize := uint64(len(m.tileBytes)) / n
			// TileWidth (322, LONG, count=1)
			_ = binary.Write(buf, binary.LittleEndian, uint16(322))
			_ = binary.Write(buf, binary.LittleEndian, uint16(4))
			_ = binary.Write(buf, binary.LittleEndian, uint64(1))
			_ = binary.Write(buf, binary.LittleEndian, m.tw)
			// TileLength (323, LONG, count=1)
			_ = binary.Write(buf, binary.LittleEndian, uint16(323))
			_ = binary.Write(buf, binary.LittleEndian, uint16(4))
			_ = binary.Write(buf, binary.LittleEndian, uint64(1))
			_ = binary.Write(buf, binary.LittleEndian, m.th)
			// TileOffsets (324, LONG8 type 16, count=n)
			_ = binary.Write(buf, binary.LittleEndian, uint16(324))
			_ = binary.Write(buf, binary.LittleEndian, uint16(16))
			_ = binary.Write(buf, binary.LittleEndian, n)
			if n == 1 {
				_ = binary.Write(buf, binary.LittleEndian, tileDataOffsets[i])
			} else {
				_ = binary.Write(buf, binary.LittleEndian, tileOffArrayOffsets[i])
			}
			// TileByteCounts (325, LONG8, count=n)
			_ = binary.Write(buf, binary.LittleEndian, uint16(325))
			_ = binary.Write(buf, binary.LittleEndian, uint16(16))
			_ = binary.Write(buf, binary.LittleEndian, n)
			if n == 1 {
				_ = binary.Write(buf, binary.LittleEndian, tileSize)
			} else {
				_ = binary.Write(buf, binary.LittleEndian, tileCntArrayOffsets[i])
			}
		}
		nextIFD := uint64(0)
		if i+1 < len(ifds) {
			nextIFD = ifdOffsets[i+1]
		}
		_ = binary.Write(buf, binary.LittleEndian, nextIFD)
	}

	// Pass 4: emit out-of-line payloads in offset-assignment order.
	for i, ifd := range ifds {
		m := &metas[i]
		if ifd.xmp != nil && len(ifd.xmp) > 8 {
			buf.Write(ifd.xmp)
		}
		if len(m.descBytes) > 8 {
			buf.Write(m.descBytes)
		}
		if ifd.tileWidth > 0 {
			n := m.gridW * m.gridH
			tileSize := uint64(len(m.tileBytes)) / n
			if n > 1 {
				// TileOffsets array
				for k := uint64(0); k < n; k++ {
					_ = binary.Write(buf, binary.LittleEndian, tileDataOffsets[i]+k*tileSize)
				}
				// TileByteCounts array
				for k := uint64(0); k < n; k++ {
					_ = binary.Write(buf, binary.LittleEndian, tileSize)
				}
			}
			buf.Write(m.tileBytes)
		}
	}
	return buf.Bytes()
}

// writeInlineOrOffset writes payload directly into the 8-byte
// value/offset cell when len(payload) <= 8, padding with zeros;
// otherwise writes the 8-byte offset to the out-of-line area.
func writeInlineOrOffset(buf *bytes.Buffer, payload []byte, offsetIfOOL uint64) {
	if len(payload) <= 8 {
		var inline [8]byte
		copy(inline[:], payload)
		buf.Write(inline[:])
		return
	}
	_ = binary.Write(buf, binary.LittleEndian, offsetIfOOL)
}

// buildClassicTIFFWithXMP builds a classic TIFF (32-bit offsets) with
// a single IFD carrying ImageWidth/ImageLength/XMP. Used to verify
// that classic-TIFF iScan files (which the spec disclaims) are
// rejected by the detection gate even when the XMP contains the
// marker substring.
func buildClassicTIFFWithXMP(t *testing.T, xmp []byte) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	// Classic TIFF header: II 42 firstIFD=0x08
	buf.Write([]byte{'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00})
	// IFD at 0x08: count(2) + 3 entries*12 + nextIFD(4) = 42 bytes
	// XMP payload sits at offset 0x08 + 42 = 0x32.
	xmpOffset := uint32(0x08 + 2 + 3*12 + 4)
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	// ImageWidth (256, SHORT, count=1, value=1024 inline u32)
	_ = binary.Write(buf, binary.LittleEndian, uint16(256))
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1024))
	// ImageLength (257, SHORT, count=1, value=768 inline u32)
	_ = binary.Write(buf, binary.LittleEndian, uint16(257))
	_ = binary.Write(buf, binary.LittleEndian, uint16(3))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))
	_ = binary.Write(buf, binary.LittleEndian, uint32(768))
	// XMP (700, UNDEFINED, count=len(xmp), value=offset u32)
	_ = binary.Write(buf, binary.LittleEndian, uint16(700))
	_ = binary.Write(buf, binary.LittleEndian, uint16(7))
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(xmp)))
	_ = binary.Write(buf, binary.LittleEndian, xmpOffset)
	// Next IFD = 0
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))
	buf.Write(xmp)
	return buf.Bytes()
}
