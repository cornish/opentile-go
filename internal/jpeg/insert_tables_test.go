package jpeg

import (
	"bytes"
	"errors"
	"testing"
)

// The exact 16-byte canonical Adobe APP14 segment Python opentile and
// Photoshop both emit (jpeg/jpeg.py:400-404 in opentile 0.20.0). Parity with
// this byte sequence is the correctness bar for InsertTablesAndAPP14.
var pythonAPP14 = []byte{
	0xFF, 0xEE, 0x00, 0x0E,
	0x41, 0x64, 0x6F, 0x62, 0x65, // "Adobe" (5 bytes, no null)
	0x00, 0x64, // DCTEncodeVersion = 100
	0x80, 0x00, // APP14Flags0 = 0x8000
	0x00, 0x00, // APP14Flags1 = 0
	0x00, // ColorTransform = 0 (RGB)
}

func TestAdobeAPP14MatchesPython(t *testing.T) {
	// Compile-time: adobeAPP14 (production constant) must equal the Python
	// byte sequence. If someone "cleans up" the bytes, this test catches it.
	if !bytes.Equal(adobeAPP14, pythonAPP14) {
		t.Fatalf("adobeAPP14 drift from Python opentile:\n got %x\nwant %x", adobeAPP14, pythonAPP14)
	}
	if len(adobeAPP14) != 16 {
		t.Fatalf("adobeAPP14 length: got %d, want 16", len(adobeAPP14))
	}
}

func TestInsertTablesAndAPP14Ordering(t *testing.T) {
	// Synthetic frame: SOI + SOF0(minimal) + SOS(minimal) + scan bytes + EOI.
	frame := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, 0x00, 0x08, 0x08, 0x00, 0x10, 0x00, 0x10, 0x03, // SOF0 stub
		0xFF, 0xDA, 0x00, 0x08, 0x03, 0x01, 0x00, 0x02, 0x11, 0x03, 0x11, // SOS stub
		0xDE, 0xAD, 0xBE, 0xEF, // scan entropy bytes
		0xFF, 0xD9, // EOI
	}
	// Synthetic JPEGTables: SOI + DQT + DHT + EOI.
	tables := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xDB, 0x00, 0x03, 0x00, // DQT stub
		0xFF, 0xC4, 0x00, 0x03, 0x10, // DHT stub
		0xFF, 0xD9, // EOI
	}
	out, err := InsertTablesAndAPP14(frame, tables)
	if err != nil {
		t.Fatalf("InsertTablesAndAPP14: %v", err)
	}
	// Expected: frame[:sosIdx] + tables[2:-2] + APP14 + frame[sosIdx:]
	// sosIdx in frame is the start of the SOS marker (after SOI+SOF0).
	sosIdx := bytes.Index(frame, []byte{0xFF, 0xDA})
	if sosIdx < 0 {
		t.Fatal("test fixture lacks SOS")
	}
	var want []byte
	want = append(want, frame[:sosIdx]...)
	want = append(want, tables[2:len(tables)-2]...)
	want = append(want, pythonAPP14...)
	want = append(want, frame[sosIdx:]...)
	if !bytes.Equal(out, want) {
		t.Errorf("output mismatch:\n got %x\nwant %x", out, want)
	}

	// Verify structural ordering: APP14 appears immediately before SOS in
	// the output, and the DQT/DHT segments appear before APP14.
	outSOS := bytes.Index(out, []byte{0xFF, 0xDA})
	outAPP14 := bytes.Index(out, pythonAPP14)
	outDQT := bytes.Index(out, []byte{0xFF, 0xDB, 0x00, 0x03, 0x00})
	outDHT := bytes.Index(out, []byte{0xFF, 0xC4, 0x00, 0x03, 0x10})
	if outDQT < 0 || outDHT < 0 || outAPP14 < 0 || outSOS < 0 {
		t.Fatalf("missing segment: dqt=%d dht=%d app14=%d sos=%d", outDQT, outDHT, outAPP14, outSOS)
	}
	if !(outDQT < outAPP14 && outDHT < outAPP14 && outAPP14 < outSOS) {
		t.Errorf("bad ordering: dqt=%d dht=%d app14=%d sos=%d", outDQT, outDHT, outAPP14, outSOS)
	}
	if outAPP14+len(pythonAPP14) != outSOS {
		t.Errorf("APP14 should sit immediately before SOS: app14_end=%d sos=%d", outAPP14+len(pythonAPP14), outSOS)
	}
}

func TestInsertTablesAndAPP14RejectsNoSOS(t *testing.T) {
	frame := []byte{0xFF, 0xD8, 0xFF, 0xD9} // SOI + EOI only
	tables := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	_, err := InsertTablesAndAPP14(frame, tables)
	if err == nil {
		t.Fatal("expected error when frame has no SOS")
	}
	if !errors.Is(err, ErrBadJPEG) {
		t.Errorf("error not wrapping ErrBadJPEG: %v", err)
	}
}

func TestInsertTablesAndAPP14RejectsShortTables(t *testing.T) {
	frame := []byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02, 0xFF, 0xD9}
	_, err := InsertTablesAndAPP14(frame, []byte{0xFF, 0xD8}) // exactly 2 bytes (too short, need >=4)
	if err == nil {
		t.Fatal("expected error on too-short tables")
	}
	if !errors.Is(err, ErrBadJPEG) {
		t.Errorf("error not wrapping ErrBadJPEG: %v", err)
	}
}

func TestInsertTablesNoAPP14(t *testing.T) {
	// Philips needs a tables-only splice (no APP14). Confirm the output
	// equals frame[:sos] + tables[2:-2] + frame[sos:] with no APP14.
	frame := []byte{
		0xFF, 0xD8,
		0xFF, 0xC0, 0x00, 0x08, 0x08, 0x00, 0x10, 0x00, 0x10, 0x03,
		0xFF, 0xDA, 0x00, 0x08, 0x03, 0x01, 0x00, 0x02, 0x11, 0x03, 0x11,
		0xDE, 0xAD,
		0xFF, 0xD9,
	}
	tables := []byte{
		0xFF, 0xD8,
		0xFF, 0xDB, 0x00, 0x03, 0x00,
		0xFF, 0xC4, 0x00, 0x03, 0x10,
		0xFF, 0xD9,
	}
	out, err := InsertTables(frame, tables)
	if err != nil {
		t.Fatalf("InsertTables: %v", err)
	}
	sosIdx := bytes.Index(frame, []byte{0xFF, 0xDA})
	var want []byte
	want = append(want, frame[:sosIdx]...)
	want = append(want, tables[2:len(tables)-2]...)
	want = append(want, frame[sosIdx:]...)
	if !bytes.Equal(out, want) {
		t.Errorf("output mismatch:\n got %x\nwant %x", out, want)
	}
	// And confirm there is NO APP14 anywhere in the output.
	if bytes.Contains(out, pythonAPP14) {
		t.Error("InsertTables must not splice APP14")
	}
}

func TestInsertTablesRejectsNoSOS(t *testing.T) {
	frame := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	tables := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	if _, err := InsertTables(frame, tables); err == nil {
		t.Fatal("expected error when frame has no SOS")
	}
}

func TestInsertTablesAndAPP14DoesNotMutateInputs(t *testing.T) {
	frame := []byte{
		0xFF, 0xD8,
		0xFF, 0xDA, 0x00, 0x02,
		0xDE, 0xAD,
		0xFF, 0xD9,
	}
	tables := []byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x03, 0x00, 0xFF, 0xD9}
	origFrame := append([]byte(nil), frame...)
	origTables := append([]byte(nil), tables...)
	if _, err := InsertTablesAndAPP14(frame, tables); err != nil {
		t.Fatalf("InsertTablesAndAPP14: %v", err)
	}
	if !bytes.Equal(frame, origFrame) {
		t.Errorf("frame was mutated")
	}
	if !bytes.Equal(tables, origTables) {
		t.Errorf("tables were mutated")
	}
}
