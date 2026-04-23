package tiff

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildNDPITIFF constructs a tiny TIFF formatted as NDPI: magic 42, classic
// 12-byte tag entries, then an 8-byte next-IFD offset (uint64), then a 4-byte
// high-bits extension per tag.
func buildNDPITIFF(t *testing.T, tags []ndpiTag) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	// Header: II 42 firstIFD (8 bytes uint64 for NDPI).
	buf.Write([]byte{'I', 'I', 42, 0})
	_ = binary.Write(buf, binary.LittleEndian, uint64(0x0C)) // firstIFD = 0x0C
	// IFD starts at 0x0C. Layout: tagno(2), 12*n tags, 8-byte next-IFD, 4*n hi-bits.
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(tags)))
	for _, tag := range tags {
		_ = binary.Write(buf, binary.LittleEndian, tag.Tag)
		_ = binary.Write(buf, binary.LittleEndian, tag.Type)
		_ = binary.Write(buf, binary.LittleEndian, uint32(tag.Count))
		_ = binary.Write(buf, binary.LittleEndian, uint32(tag.ValueOrOffset&0xFFFFFFFF))
	}
	// Next-IFD (uint64, 0 = end of chain)
	_ = binary.Write(buf, binary.LittleEndian, uint64(0))
	// High-bits extension: 4 bytes per tag.
	for _, tag := range tags {
		_ = binary.Write(buf, binary.LittleEndian, uint32(tag.ValueOrOffset>>32))
	}
	return buf.Bytes()
}

type ndpiTag struct {
	Tag           uint16
	Type          DataType
	Count         uint64
	ValueOrOffset uint64
}

func TestWalkNDPIIFDs(t *testing.T) {
	// Build an IFD with one tag whose value is 0x100000000 (just past 4 GiB).
	// The low 32 bits are 0, high 32 bits are 1.
	data := buildNDPITIFF(t, []ndpiTag{
		{Tag: 324, Type: DTLong, Count: 1, ValueOrOffset: 0x100000000}, // TileOffsets = 4 GiB
		{Tag: 65420, Type: DTLong, Count: 1, ValueOrOffset: 20},        // SourceLens = 20 (inline)
	})
	r := bytes.NewReader(data)
	h, err := parseHeader(r)
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	b := newByteReader(r, h.littleEndian)
	ifds, err := walkNDPIIFDs(b, int64(h.firstIFD))
	if err != nil {
		t.Fatalf("walkNDPIIFDs: %v", err)
	}
	if len(ifds) != 1 {
		t.Fatalf("ifd count: got %d", len(ifds))
	}
	e, ok := ifds[0].get(324)
	if !ok {
		t.Fatal("tag 324 missing")
	}
	if e.valueOrOffset != 0x100000000 {
		t.Errorf("valueOrOffset: got 0x%x, want 0x100000000", e.valueOrOffset)
	}
	if e.inlineCap != 4 {
		t.Errorf("inlineCap: got %d, want 4 (NDPI is classic-width inline)", e.inlineCap)
	}
	// SourceLens should be readable via Values.
	sl, ok := ifds[0].get(65420)
	if !ok {
		t.Fatal("SourceLens missing")
	}
	vals, err := sl.Values(b)
	if err != nil || len(vals) != 1 || vals[0] != 20 {
		t.Fatalf("SourceLens: got %v, err %v", vals, err)
	}
}

func TestNDPISniffAutoDetect(t *testing.T) {
	// An NDPI file with SourceLens tag; File.Open should detect NDPI mode
	// and parse IFDs correctly, making tag values with high bits readable.
	data := buildNDPITIFF(t, []ndpiTag{
		{Tag: 256, Type: DTShort, Count: 1, ValueOrOffset: 1024},       // ImageWidth (inline)
		{Tag: 65420, Type: DTLong, Count: 1, ValueOrOffset: 20},        // SourceLens
		{Tag: 324, Type: DTLong, Count: 1, ValueOrOffset: 0x100000000}, // TileOffsets at 4 GiB
	})
	f, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !f.NDPI() {
		t.Error("expected NDPI()=true after SourceLens sniff")
	}
	p := f.Pages()[0]
	offs, err := p.TileOffsets64()
	if err != nil {
		t.Fatalf("TileOffsets64: %v", err)
	}
	if len(offs) != 1 || offs[0] != 0x100000000 {
		t.Errorf("TileOffsets64: got %v, want [0x100000000]", offs)
	}
}
