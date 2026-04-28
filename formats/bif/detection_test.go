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
		{description: "level=0 mag=40 quality=95"},
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
// BigTIFF. ImageWidth/ImageLength are fixed at 1024×768; XMP and
// ImageDescription are optional. Used by detection / classification
// / layout tests; downstream tile-reader tests will need richer
// fixtures with TileOffsets / TileByteCounts / JPEGTables.
type iFDSpec struct {
	xmp         []byte // nil = omit XMP tag (700)
	description string // empty = omit ImageDescription tag (270)
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

	// Two passes. Pass 1 lays out IFDs head-to-toe and reserves
	// out-of-line payload areas for any field too large to fit in the
	// 8-byte value/offset cell. Pass 2 actually writes everything.
	//
	// ImageDescription is type 2 (ASCII, NUL-terminated). We store the
	// string + a trailing NUL — the spec requires the NUL.
	ifdOffsets := make([]uint64, len(ifds))
	xmpOffsets := make([]uint64, len(ifds))
	descOffsets := make([]uint64, len(ifds))
	descBytes := make([][]byte, len(ifds))
	cursor := uint64(0x10)
	for i, ifd := range ifds {
		ifdOffsets[i] = cursor
		entryCount := uint64(2) // ImageWidth + ImageLength
		if ifd.xmp != nil {
			entryCount++
		}
		if ifd.description != "" {
			entryCount++
			descBytes[i] = append([]byte(ifd.description), 0)
		}
		cursor += 8 + entryCount*20 + 8
	}
	for i, ifd := range ifds {
		if ifd.xmp != nil && len(ifd.xmp) > 8 {
			xmpOffsets[i] = cursor
			cursor += uint64(len(ifd.xmp))
		}
		if len(descBytes[i]) > 8 {
			descOffsets[i] = cursor
			cursor += uint64(len(descBytes[i]))
		}
		_ = ifd
	}

	for i, ifd := range ifds {
		entries := uint64(2)
		if ifd.xmp != nil {
			entries++
		}
		if ifd.description != "" {
			entries++
		}
		_ = binary.Write(buf, binary.LittleEndian, entries)
		// ImageWidth (256, SHORT, count=1, value=1024 inline)
		_ = binary.Write(buf, binary.LittleEndian, uint16(256))
		_ = binary.Write(buf, binary.LittleEndian, uint16(3))
		_ = binary.Write(buf, binary.LittleEndian, uint64(1))
		_ = binary.Write(buf, binary.LittleEndian, uint64(1024))
		// ImageLength (257, SHORT, count=1, value=768 inline)
		_ = binary.Write(buf, binary.LittleEndian, uint16(257))
		_ = binary.Write(buf, binary.LittleEndian, uint16(3))
		_ = binary.Write(buf, binary.LittleEndian, uint64(1))
		_ = binary.Write(buf, binary.LittleEndian, uint64(768))
		if ifd.description != "" {
			// ImageDescription (270, ASCII, count=len(descBytes))
			_ = binary.Write(buf, binary.LittleEndian, uint16(270))
			_ = binary.Write(buf, binary.LittleEndian, uint16(2))
			_ = binary.Write(buf, binary.LittleEndian, uint64(len(descBytes[i])))
			if len(descBytes[i]) <= 8 {
				var inline [8]byte
				copy(inline[:], descBytes[i])
				buf.Write(inline[:])
			} else {
				_ = binary.Write(buf, binary.LittleEndian, descOffsets[i])
			}
		}
		if ifd.xmp != nil {
			// XMP (700, UNDEFINED, count=len(xmp))
			_ = binary.Write(buf, binary.LittleEndian, uint16(700))
			_ = binary.Write(buf, binary.LittleEndian, uint16(7))
			_ = binary.Write(buf, binary.LittleEndian, uint64(len(ifd.xmp)))
			if len(ifd.xmp) <= 8 {
				var inline [8]byte
				copy(inline[:], ifd.xmp)
				buf.Write(inline[:])
			} else {
				_ = binary.Write(buf, binary.LittleEndian, xmpOffsets[i])
			}
		}
		// Next IFD pointer (or 0 for last IFD).
		nextIFD := uint64(0)
		if i+1 < len(ifds) {
			nextIFD = ifdOffsets[i+1]
		}
		_ = binary.Write(buf, binary.LittleEndian, nextIFD)
	}

	// Out-of-line payload area: XMP first, then ImageDescription, in
	// the same per-IFD interleaved order assigned during pass 1.
	for i, ifd := range ifds {
		if ifd.xmp != nil && len(ifd.xmp) > 8 {
			buf.Write(ifd.xmp)
		}
		if len(descBytes[i]) > 8 {
			buf.Write(descBytes[i])
		}
	}
	return buf.Bytes()
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
