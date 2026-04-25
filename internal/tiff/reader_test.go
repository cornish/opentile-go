package tiff

import (
	"bytes"
	"testing"
)

func TestByteReader(t *testing.T) {
	// little-endian: u16=0x0201 at offset 0; u32=0x06050403 at offset 2
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	r := bytes.NewReader(data)

	bl := newByteReader(r, true)
	v16, err := bl.uint16(0)
	if err != nil || v16 != 0x0201 {
		t.Fatalf("uint16 LE: got 0x%x, err %v; want 0x0201", v16, err)
	}
	v32, err := bl.uint32(2)
	if err != nil || v32 != 0x06050403 {
		t.Fatalf("uint32 LE: got 0x%x, err %v; want 0x06050403", v32, err)
	}

	bb := newByteReader(r, false)
	v16b, _ := bb.uint16(0)
	if v16b != 0x0102 {
		t.Fatalf("uint16 BE: got 0x%x, want 0x0102", v16b)
	}
	v32b, _ := bb.uint32(2)
	if v32b != 0x03040506 {
		t.Fatalf("uint32 BE: got 0x%x, want 0x03040506", v32b)
	}
}

func TestByteReaderShort(t *testing.T) {
	r := bytes.NewReader([]byte{0x01})
	b := newByteReader(r, true)
	if _, err := b.uint16(0); err == nil {
		t.Fatal("expected error for short read")
	}
}

func TestByteReaderUint64(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	r := bytes.NewReader(data)
	bl := newByteReader(r, true)
	v, err := bl.uint64(0)
	if err != nil || v != 0x0807060504030201 {
		t.Fatalf("uint64 LE: got %x, err %v; want 0x0807060504030201", v, err)
	}
	bb := newByteReader(r, false)
	v, err = bb.uint64(0)
	if err != nil || v != 0x0102030405060708 {
		t.Fatalf("uint64 BE: got %x, err %v", v, err)
	}
}
