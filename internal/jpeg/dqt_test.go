package jpeg

import (
	"testing"
)

// Simple synthetic JPEG header with one DQT (table 0, precision 8, 64-byte
// payload: DC=16, rest zero). Verifies LumaDCQuant picks up the DC value.
func TestLumaDCQuant8Bit(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8) // SOI
	// DQT segment length = 2 (length field) + 1 (precision/id) + 64 (table) = 67.
	buf = append(buf, 0xFF, 0xDB, 0x00, 67)
	buf = append(buf, 0x00) // precision=0, id=0
	buf = append(buf, 16)   // DC element = 16
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9) // EOI

	got, err := LumaDCQuant(buf)
	if err != nil {
		t.Fatalf("LumaDCQuant: %v", err)
	}
	if got != 16 {
		t.Errorf("LumaDCQuant = %d, want 16", got)
	}
}

// Precision-1 (16-bit) DQT: table 0, DC = 0x0101 = 257.
func TestLumaDCQuant16Bit(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	// Length = 2 + 1 + 128 = 131.
	buf = append(buf, 0xFF, 0xDB, 0x00, 131)
	buf = append(buf, 0x10) // precision=1 (upper nibble), id=0
	buf = append(buf, 0x01, 0x01)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	got, err := LumaDCQuant(buf)
	if err != nil {
		t.Fatalf("LumaDCQuant: %v", err)
	}
	if got != 257 {
		t.Errorf("LumaDCQuant = %d, want 257", got)
	}
}

// DQT for chroma table appears FIRST but we want table 0 — the parser must
// skip to the next DQT.
func TestLumaDCQuantSkipsOtherTables(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	// Chroma DQT: length=67, precision=0 id=1, DC=7.
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x01, 7)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	// Luma DQT: length=67, precision=0 id=0, DC=12.
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x00, 12)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	got, err := LumaDCQuant(buf)
	if err != nil {
		t.Fatalf("LumaDCQuant: %v", err)
	}
	if got != 12 {
		t.Errorf("LumaDCQuant = %d, want 12", got)
	}
}

// Luminance=1.0 with dc_quant=8 → round((2047-1024)/8) = round(127.875) = 128.
func TestLuminanceToDCCoefficientWhite(t *testing.T) {
	// Build JPEG with luma DC quant = 8.
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x00, 8)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	got, err := LuminanceToDCCoefficient(buf, 1.0)
	if err != nil {
		t.Fatalf("LuminanceToDCCoefficient: %v", err)
	}
	// Python's round(127.875) = 128 (banker's rounding, not half-away-from-zero).
	if got != 128 {
		t.Errorf("white-fill DC for dc_quant=8: got %d, want 128", got)
	}
}

// Luminance=0 → round((-1024)/8) = -128.
func TestLuminanceToDCCoefficientBlack(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x00, 8)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	got, err := LuminanceToDCCoefficient(buf, 0.0)
	if err != nil {
		t.Fatalf("LuminanceToDCCoefficient: %v", err)
	}
	if got != -128 {
		t.Errorf("black-fill DC for dc_quant=8: got %d, want -128", got)
	}
}

// Luminance=0.5 → round(-0.5/8) = round(-0.0625) = 0.
func TestLuminanceToDCCoefficientGray(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x00, 8)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	got, err := LuminanceToDCCoefficient(buf, 0.5)
	if err != nil {
		t.Fatalf("LuminanceToDCCoefficient: %v", err)
	}
	if got != 0 {
		t.Errorf("gray-fill DC for dc_quant=8: got %d, want 0", got)
	}
}

// Tie at exactly 0.5 should round to even per Python's built-in round().
// luminance=1.0 and dc_quant=2: (2047-1024)/2 = 511.5 → round-to-even = 512.
func TestLuminanceToDCCoefficientTiesEven(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x00, 2)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	got, err := LuminanceToDCCoefficient(buf, 1.0)
	if err != nil {
		t.Fatalf("LuminanceToDCCoefficient: %v", err)
	}
	// 511.5 is a tie; Python's round(511.5) = 512 (even).
	if got != 512 {
		t.Errorf("tie case (511.5): got %d, want 512 (round-to-even)", got)
	}
}

// Luminance clamped above 1.0 should behave as 1.0.
func TestLuminanceToDCCoefficientClamped(t *testing.T) {
	var buf []byte
	buf = append(buf, 0xFF, 0xD8)
	buf = append(buf, 0xFF, 0xDB, 0x00, 67, 0x00, 8)
	for i := 1; i < 64; i++ {
		buf = append(buf, 0)
	}
	buf = append(buf, 0xFF, 0xD9)

	a, _ := LuminanceToDCCoefficient(buf, 2.0)
	b, _ := LuminanceToDCCoefficient(buf, 1.0)
	if a != b {
		t.Errorf("clamped >1 (%d) should equal 1.0 (%d)", a, b)
	}
	c, _ := LuminanceToDCCoefficient(buf, -1.0)
	d, _ := LuminanceToDCCoefficient(buf, 0.0)
	if c != d {
		t.Errorf("clamped <0 (%d) should equal 0.0 (%d)", c, d)
	}
}
