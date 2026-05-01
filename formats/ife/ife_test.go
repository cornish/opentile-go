package ife

import (
	"bytes"
	"errors"
	"testing"

	opentile "github.com/cornish/opentile-go"
)

func TestSupportsRaw(t *testing.T) {
	f := New()

	// Magic match.
	good := append([]byte{0x73, 0x69, 0x72, 0x49}, make([]byte, 4)...)
	if !f.SupportsRaw(bytes.NewReader(good), int64(len(good))) {
		t.Error("good magic: SupportsRaw returned false")
	}

	// Wrong magic.
	bad := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0, 0, 0, 0}
	if f.SupportsRaw(bytes.NewReader(bad), int64(len(bad))) {
		t.Error("wrong magic: SupportsRaw returned true")
	}

	// Too small.
	tiny := []byte{0x73, 0x69}
	if f.SupportsRaw(bytes.NewReader(tiny), int64(len(tiny))) {
		t.Error("tiny file: SupportsRaw returned true")
	}

	// Empty.
	if f.SupportsRaw(bytes.NewReader(nil), 0) {
		t.Error("empty file: SupportsRaw returned true")
	}
}

func TestOpenRawStub(t *testing.T) {
	// Until T11 wires the Tiler, OpenRaw on a magic-bearing buffer
	// returns errIFETilerUnimplemented. Pin the contract so removing
	// the stub later doesn't accidentally regress to silent success.
	f := New()
	good := append([]byte{0x73, 0x69, 0x72, 0x49}, make([]byte, 4)...)
	_, err := f.OpenRaw(bytes.NewReader(good), int64(len(good)), &opentile.Config{})
	if !errors.Is(err, errIFETilerUnimplemented) {
		t.Errorf("OpenRaw stub: got %v, want errIFETilerUnimplemented", err)
	}
}

func TestSupports_TIFFPathAlwaysFalse(t *testing.T) {
	if New().Supports(nil) {
		t.Error("Supports(*tiff.File) should always return false on IFE")
	}
}

func TestFormat(t *testing.T) {
	if got := New().Format(); got != opentile.FormatIFE {
		t.Errorf("Format() = %v, want %v", got, opentile.FormatIFE)
	}
}
