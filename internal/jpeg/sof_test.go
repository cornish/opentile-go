package jpeg

import (
	"bytes"
	"testing"
)

func TestParseSOFYCbCr420(t *testing.T) {
	// SOF0 payload: precision=8, height=0x0200 (512), width=0x0300 (768),
	// 3 components, each: id, sampling (H<<4|V), quant-id.
	// Y: H=2 V=2, Cb: 1/1, Cr: 1/1 → 4:2:0 subsampling.
	payload := []byte{
		0x08,             // precision
		0x02, 0x00,       // height 512
		0x03, 0x00,       // width 768
		0x03,             // 3 components
		0x01, 0x22, 0x00, // Y id=1 H=2 V=2 qt=0
		0x02, 0x11, 0x01, // Cb id=2 H=1 V=1 qt=1
		0x03, 0x11, 0x01, // Cr id=3 H=1 V=1 qt=1
	}
	sof, err := ParseSOF(payload)
	if err != nil {
		t.Fatalf("ParseSOF: %v", err)
	}
	if sof.Width != 768 || sof.Height != 512 {
		t.Errorf("dims: got %dx%d, want 768x512", sof.Width, sof.Height)
	}
	if len(sof.Components) != 3 {
		t.Fatalf("components: got %d, want 3", len(sof.Components))
	}
	if sof.Components[0].SamplingH != 2 || sof.Components[0].SamplingV != 2 {
		t.Errorf("Y sampling: got %d/%d, want 2/2",
			sof.Components[0].SamplingH, sof.Components[0].SamplingV)
	}
	mcuW, mcuH := sof.MCUSize()
	if mcuW != 16 || mcuH != 16 {
		t.Errorf("MCU: got %dx%d, want 16x16 (4:2:0)", mcuW, mcuH)
	}
}

func TestBuildSOFRoundTrip(t *testing.T) {
	want := &SOF{
		Precision: 8, Width: 512, Height: 256,
		Components: []SOFComponent{
			{ID: 1, SamplingH: 1, SamplingV: 1, QuantTableID: 0},
			{ID: 2, SamplingH: 1, SamplingV: 1, QuantTableID: 0},
			{ID: 3, SamplingH: 1, SamplingV: 1, QuantTableID: 0},
		},
	}
	seg := BuildSOF(want)
	// Verify seg begins with FF C0 and the length field is consistent.
	if seg[0] != 0xFF || Marker(seg[1]) != SOF0 {
		t.Fatalf("marker: got %x %x, want FF C0", seg[0], seg[1])
	}
	length := int(seg[2])<<8 | int(seg[3])
	wantLen := 2 /*length bytes*/ + 6 /*fixed*/ + 3*len(want.Components)
	if length != wantLen {
		t.Errorf("length: got %d, want %d", length, wantLen)
	}
	// Strip marker+length, parse back, compare structurally.
	got, err := ParseSOF(seg[4:])
	if err != nil {
		t.Fatalf("ParseSOF round-trip: %v", err)
	}
	if got.Width != want.Width || got.Height != want.Height {
		t.Errorf("dims drift: got %dx%d", got.Width, got.Height)
	}
	if !bytes.Equal(seg, BuildSOF(got)) {
		t.Error("BuildSOF not deterministic on round-trip")
	}
}

func TestReplaceSOFDimensions(t *testing.T) {
	// Full minimal JPEG: SOI + SOF0(512x256) + SOS(empty) + EOI.
	sof := BuildSOF(&SOF{
		Precision: 8, Width: 512, Height: 256,
		Components: []SOFComponent{
			{ID: 1, SamplingH: 1, SamplingV: 1, QuantTableID: 0},
		},
	})
	jpg := append([]byte{0xFF, 0xD8}, sof...)
	jpg = append(jpg, 0xFF, 0xDA, 0x00, 0x08, 1, 1, 0, 0, 0x3F, 0x00) // SOS
	jpg = append(jpg, 0xFF, 0xD9)

	got, err := ReplaceSOFDimensions(jpg, 1024, 768)
	if err != nil {
		t.Fatalf("ReplaceSOFDimensions: %v", err)
	}
	// Find the new SOF, parse it, confirm dims.
	var newSOF *SOF
	for seg, err := range Scan(bytes.NewReader(got)) {
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if seg.Marker == SOF0 {
			newSOF, _ = ParseSOF(seg.Payload)
			break
		}
	}
	if newSOF == nil {
		t.Fatal("SOF not found in rewritten JPEG")
	}
	if newSOF.Width != 1024 || newSOF.Height != 768 {
		t.Errorf("dims: got %dx%d, want 1024x768", newSOF.Width, newSOF.Height)
	}
}

func TestReplaceSOFDimensionsRejectsMissingSOF(t *testing.T) {
	jpg := []byte{0xFF, 0xD8, 0xFF, 0xD9} // SOI + EOI, no SOF
	_, err := ReplaceSOFDimensions(jpg, 1, 1)
	if err == nil {
		t.Fatal("expected error when no SOF present")
	}
}
