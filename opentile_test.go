package opentile

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tcornish/opentile-go/internal/tiff"
)

// fakeFactory is a test double that reports support when the tag
// ImageDescription begins with "FAKE".
type fakeFactory struct{ openCalled bool }

func (f *fakeFactory) Format() Format { return Format("fake") }
func (f *fakeFactory) Supports(file *tiff.File) bool {
	if len(file.Pages()) == 0 {
		return false
	}
	desc, _ := file.Pages()[0].ImageDescription()
	return len(desc) >= 4 && desc[:4] == "FAKE"
}
func (f *fakeFactory) Open(file *tiff.File, cfg *Config) (Tiler, error) {
	f.openCalled = true
	return &noopTiler{format: Format("fake")}, nil
}

type noopTiler struct {
	format Format
}

func (n *noopTiler) Format() Format                 { return n.format }
func (n *noopTiler) Levels() []Level                { return nil }
func (n *noopTiler) Level(i int) (Level, error)     { return nil, ErrLevelOutOfRange }
func (n *noopTiler) Associated() []AssociatedImage  { return nil }
func (n *noopTiler) Metadata() Metadata             { return Metadata{} }
func (n *noopTiler) ICCProfile() []byte             { return nil }
func (n *noopTiler) Close() error                   { return nil }

func TestRegisterAndOpen(t *testing.T) {
	// Reset registry for test isolation.
	resetRegistry()
	f := &fakeFactory{}
	Register(f)

	data := buildTIFFWithDescription(t, "FAKE slide")
	tiler, err := Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer tiler.Close()
	if !f.openCalled {
		t.Fatal("factory.Open was not called")
	}
	if tiler.Format() != Format("fake") {
		t.Fatalf("Format: got %q", tiler.Format())
	}
}

func TestOpenUnsupported(t *testing.T) {
	resetRegistry()
	data := buildTIFFWithDescription(t, "UNKNOWN")
	_, err := Open(bytes.NewReader(data), int64(len(data)))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestOpenInvalidTIFF(t *testing.T) {
	resetRegistry()
	_, err := Open(bytes.NewReader([]byte{'X', 'Y'}), 2)
	if !errors.Is(err, ErrInvalidTIFF) {
		t.Fatalf("expected ErrInvalidTIFF, got %v", err)
	}
}

func TestOpenFileErrorIncludesPath(t *testing.T) {
	_, err := OpenFile("/nonexistent/slide.svs")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
	if !strings.Contains(err.Error(), "/nonexistent/slide.svs") {
		t.Errorf("error should include path: %v", err)
	}
}

// buildTIFFWithDescription creates a 1-IFD TIFF whose ImageDescription is desc.
// Minimal: ImageWidth, ImageLength, TileWidth, TileLength, ImageDescription.
func buildTIFFWithDescription(t *testing.T, desc string) []byte {
	t.Helper()
	// 5 entries: 256, 257, 270 (ASCII), 322, 323.
	buf := new(bytes.Buffer)
	buf.Write([]byte{'I', 'I', 42, 0, 0x08, 0, 0, 0})
	// IFD at 8: count(2) + 5*12 + 4 = 66 bytes → external base at 8+66 = 74.
	descBytes := append([]byte(desc), 0) // NUL terminate
	descOff := uint32(74)
	// entries
	writeU16 := func(v uint16) { buf.WriteByte(byte(v)); buf.WriteByte(byte(v >> 8)) }
	writeU32 := func(v uint32) {
		buf.WriteByte(byte(v))
		buf.WriteByte(byte(v >> 8))
		buf.WriteByte(byte(v >> 16))
		buf.WriteByte(byte(v >> 24))
	}
	writeU16(5)
	// ImageWidth=1024
	writeU16(256); writeU16(3); writeU32(1); writeU32(1024)
	// ImageLength=768
	writeU16(257); writeU16(3); writeU32(1); writeU32(768)
	// ImageDescription
	writeU16(270); writeU16(2); writeU32(uint32(len(descBytes))); writeU32(descOff)
	// TileWidth=256
	writeU16(322); writeU16(3); writeU32(1); writeU32(256)
	// TileLength=256
	writeU16(323); writeU16(3); writeU32(1); writeU32(256)
	writeU32(0) // next IFD
	buf.Write(descBytes)
	return buf.Bytes()
}
