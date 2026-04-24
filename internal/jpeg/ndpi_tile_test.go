package jpeg

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildNDPIPrefix synthesizes a minimal JPEG header prefix as NDPI would
// store one: SOI, DRI, SOF0, SOS, then one byte of "scan entropy data"
// (which is where NDPI's actual scan payload begins in the file). The
// returned bytes are truncated at the first scan byte, matching what
// tifffile reads via fh.read(mcustarts[0]).
func buildNDPIPrefix(t *testing.T, restartInterval, imgW, imgH int, sampH, sampV uint8) []byte {
	t.Helper()
	var buf bytes.Buffer
	// SOI
	buf.Write([]byte{0xFF, 0xD8})
	// DRI: FF DD | length=4 | interval (u16)
	buf.Write([]byte{0xFF, 0xDD, 0x00, 0x04})
	_ = binary.Write(&buf, binary.BigEndian, uint16(restartInterval))
	// SOF0: FF C0 | length = 2 + 6 + 3*n; n=3 → length=17
	// payload: precision(1) height(2) width(2) ncomponents(1) then 3*3 component bytes
	buf.Write([]byte{0xFF, 0xC0, 0x00, 0x11, 0x08})
	_ = binary.Write(&buf, binary.BigEndian, uint16(imgH))
	_ = binary.Write(&buf, binary.BigEndian, uint16(imgW))
	buf.WriteByte(0x03)
	// Y component: id=1, sampling=(sampH<<4)|sampV, qtable=0
	buf.Write([]byte{0x01, byte(sampH<<4) | sampV, 0x00})
	// Cb: id=2, sampling=0x11, qtable=1
	buf.Write([]byte{0x02, 0x11, 0x01})
	// Cr: id=3, sampling=0x11, qtable=1
	buf.Write([]byte{0x03, 0x11, 0x01})
	// SOS: FF DA | length = 2 + 1 + 2*n + 3 = 12 for n=3
	buf.Write([]byte{0xFF, 0xDA, 0x00, 0x0C, 0x03})
	buf.Write([]byte{0x01, 0x00}) // Y: Td/Ta
	buf.Write([]byte{0x02, 0x11}) // Cb
	buf.Write([]byte{0x03, 0x11}) // Cr
	buf.Write([]byte{0x00, 0x3F, 0x00}) // Ss, Se, Ah/Al
	// Simulate "one byte of scan data" that should NOT appear in the patched
	// header; NDPIStripeJPEGHeader must stop at the SOS payload boundary.
	buf.WriteByte(0xAA)
	return buf.Bytes()
}

func TestNDPIStripeJPEGHeader_Basic(t *testing.T) {
	// YCbCr 4:2:0 → Y sampling 2x2, Cb/Cr 1x1 → MCU 16×16.
	// DRI = 40 → stripeW = 40 × 16 = 640, stripeH = 16.
	prefix := buildNDPIPrefix(t, 40, 51200, 38144, 2, 2)
	sw, sh, header, err := NDPIStripeJPEGHeader(prefix)
	if err != nil {
		t.Fatalf("NDPIStripeJPEGHeader: %v", err)
	}
	if sw != 640 || sh != 16 {
		t.Errorf("stripe size: got %dx%d, want 640x16", sw, sh)
	}
	// Patched header must be exactly sosOffset bytes (trailing 0xAA dropped).
	if len(header) != len(prefix)-1 {
		t.Errorf("header length: got %d, want %d (prefix minus trailing scan byte)", len(header), len(prefix)-1)
	}
	if header[len(header)-1] == 0xAA {
		t.Error("patched header should not include scan entropy byte")
	}
	// SOF dims: height should now be 16 (stripeH), width 640 (stripeW).
	// Locate SOF0.
	sof := bytes.Index(header, []byte{0xFF, 0xC0})
	if sof < 0 {
		t.Fatal("patched header missing SOF0 marker")
	}
	// SOF layout: FF C0 L L P H H W W ... → sof+5..sof+9 is H(2) W(2).
	gotH := binary.BigEndian.Uint16(header[sof+5 : sof+7])
	gotW := binary.BigEndian.Uint16(header[sof+7 : sof+9])
	if gotH != 16 || gotW != 640 {
		t.Errorf("patched SOF dims: got %dx%d, want 16x640", gotW, gotH)
	}
}

func TestNDPIStripeJPEGHeader_Missing(t *testing.T) {
	// Prefix with SOI only → missing DRI/SOF/SOS → error.
	_, _, _, err := NDPIStripeJPEGHeader([]byte{0xFF, 0xD8})
	if err == nil {
		t.Fatal("expected error on missing markers")
	}
}
