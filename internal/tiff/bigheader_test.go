package tiff

import (
	"bytes"
	"errors"
	"testing"
)

func TestParseBigTIFFHeader(t *testing.T) {
	// BigTIFF LE: II(4949) 43(2B00) offsetSize(0800) constant(0000) firstIFD(uint64)
	// Place firstIFD at 0x10.
	data := []byte{
		'I', 'I',
		0x2B, 0x00, // magic 43
		0x08, 0x00, // offset size = 8
		0x00, 0x00, // constant
		0x10, 0, 0, 0, 0, 0, 0, 0, // firstIFD = 0x10 (uint64 LE)
	}
	h, err := parseHeader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseHeader: %v", err)
	}
	if !h.littleEndian || !h.bigTIFF || h.firstIFD != 0x10 {
		t.Fatalf("header: got %+v", h)
	}
}

func TestParseBigTIFFRejectsBadOffsetSize(t *testing.T) {
	data := []byte{
		'I', 'I',
		0x2B, 0x00,
		0x04, 0x00, // bad offset size (should be 8)
		0x00, 0x00,
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	_, err := parseHeader(bytes.NewReader(data))
	if !errors.Is(err, ErrInvalidTIFF) {
		t.Fatalf("expected ErrInvalidTIFF, got %v", err)
	}
}

func TestParseBigTIFFRejectsBadConstant(t *testing.T) {
	data := []byte{
		'I', 'I',
		0x2B, 0x00,
		0x08, 0x00,
		0xFF, 0xFF, // bad constant
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	_, err := parseHeader(bytes.NewReader(data))
	if !errors.Is(err, ErrInvalidTIFF) {
		t.Fatalf("expected ErrInvalidTIFF, got %v", err)
	}
}
