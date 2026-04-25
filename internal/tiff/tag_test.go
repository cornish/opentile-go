package tiff

import (
	"bytes"
	"testing"
)

func TestDataTypeSize(t *testing.T) {
	tests := []struct {
		dt   DataType
		want int
	}{
		{DTByte, 1},
		{DTASCII, 1},
		{DTShort, 2},
		{DTLong, 4},
		{DTRational, 8},
		{DTUndefined, 1},
	}
	for _, tt := range tests {
		if got := tt.dt.Size(); got != tt.want {
			t.Errorf("%v.Size() = %d, want %d", tt.dt, got, tt.want)
		}
	}
}

func TestTagValueDecodeInline(t *testing.T) {
	// Entry with count=2 SHORT values stored inline: values 100, 200.
	// Inline 4-byte cell (LE): [0x64,0x00,0xC8,0x00]
	data := []byte{0x64, 0x00, 0xC8, 0x00}
	r := bytes.NewReader(data)
	b := newByteReader(r, true)

	entry := Entry{Tag: 256, Type: DTShort, Count: 2, valueOrOffset: 0}
	// override: put the payload at offset 0 for this test
	vals, err := entry.decodeInline(b, data)
	if err != nil {
		t.Fatalf("decodeInline: %v", err)
	}
	if len(vals) != 2 || vals[0] != 100 || vals[1] != 200 {
		t.Fatalf("vals: got %v, want [100 200]", vals)
	}
}

func TestTagValueDecodeExternal(t *testing.T) {
	// Entry count=2 LONG (8 bytes), value stored at offset 16.
	// LE: [0x0A,0,0,0, 0x14,0,0,0] → values 10, 20
	data := make([]byte, 24)
	copy(data[16:], []byte{0x0A, 0, 0, 0, 0x14, 0, 0, 0})
	r := bytes.NewReader(data)
	b := newByteReader(r, true)

	entry := Entry{Tag: 324, Type: DTLong, Count: 2, valueOrOffset: 16}
	vals, err := entry.decodeExternal(b)
	if err != nil {
		t.Fatalf("decodeExternal: %v", err)
	}
	if len(vals) != 2 || vals[0] != 10 || vals[1] != 20 {
		t.Fatalf("vals: got %v, want [10 20]", vals)
	}
}

func TestDecodeASCII(t *testing.T) {
	// "Aperio\x00" (7 bytes) stored externally.
	data := make([]byte, 16)
	copy(data[8:], []byte("Aperio\x00"))
	r := bytes.NewReader(data)
	b := newByteReader(r, true)

	entry := Entry{Tag: 270, Type: DTASCII, Count: 7, valueOrOffset: 8}
	s, err := entry.decodeASCII(b, data[:8]) // inline cell unused here
	if err != nil {
		t.Fatalf("decodeASCII: %v", err)
	}
	if s != "Aperio" {
		t.Fatalf("got %q, want %q", s, "Aperio")
	}
}

func TestDecodeBufferShortBuffer(t *testing.T) {
	// Entry claims 3 DTShort values (6 bytes) but buffer is only 4.
	// Without the bounds guard, this would panic in binary.ByteOrder.Uint16.
	r := bytes.NewReader(make([]byte, 4))
	b := newByteReader(r, true)
	entry := Entry{Tag: 256, Type: DTShort, Count: 3}
	_, err := entry.decodeBuffer(b, make([]byte, 4))
	if err == nil {
		t.Fatal("expected error for buffer shorter than Count*Size")
	}
}

func TestDecodeInlineRejectsOversize(t *testing.T) {
	// DTLong Count=2 needs 8 bytes but inline cell is 4; fitsInline false,
	// and decodeInline must reject rather than pass through.
	data := []byte{0, 0, 0, 0}
	r := bytes.NewReader(data)
	b := newByteReader(r, true)
	entry := Entry{Tag: 324, Type: DTLong, Count: 2}
	_, err := entry.decodeInline(b, data)
	if err == nil {
		t.Fatal("expected decodeInline to reject oversize value")
	}
}

func TestDataTypeSizeV02(t *testing.T) {
	tests := []struct {
		dt   DataType
		want int
	}{
		{DTLong8, 8},
		{DTIFD, 4},
		{DTIFD8, 8},
	}
	for _, tt := range tests {
		if got := tt.dt.Size(); got != tt.want {
			t.Errorf("%v.Size() = %d, want %d", tt.dt, got, tt.want)
		}
	}
}

func TestDecodeLong8(t *testing.T) {
	// Entry count=2 LONG8 (16 bytes), stored at offset 16.
	data := make([]byte, 32)
	copy(data[16:], []byte{
		0x01, 0, 0, 0, 0, 0, 0, 0,
		0x02, 0, 0, 0, 0, 0, 0, 0,
	})
	r := bytes.NewReader(data)
	b := newByteReader(r, true)
	entry := Entry{Tag: 324, Type: DTLong8, Count: 2, valueOrOffset: 16}
	vals, err := entry.Values64(b)
	if err != nil {
		t.Fatalf("Values64: %v", err)
	}
	if len(vals) != 2 || vals[0] != 1 || vals[1] != 2 {
		t.Fatalf("vals: got %v, want [1 2]", vals)
	}
}
