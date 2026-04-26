package tifflzw

import (
	"bytes"
	"io"
	"testing"
)

// TestRoundTrip exercises the writer/reader pair end to end across data
// large enough to force at least one code-width transition (otherwise the
// "off by one" branch in incHi never executes).
func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"short", []byte("hello world")},
		{"zeros-1KB", bytes.Repeat([]byte{0x00}, 1024)},
		{"sequential-4KB", func() []byte {
			out := make([]byte, 4096)
			for i := range out {
				out[i] = byte(i)
			}
			return out
		}()},
		{"alternating-2KB", func() []byte {
			out := make([]byte, 2048)
			for i := range out {
				if i%2 == 0 {
					out[i] = 0xAA
				} else {
					out[i] = 0x55
				}
			}
			return out
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewWriter(&buf, MSB, 8)
			if _, err := w.Write(tc.data); err != nil {
				t.Fatalf("write: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
			r := NewReader(bytes.NewReader(buf.Bytes()), MSB, 8)
			defer r.Close()
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if !bytes.Equal(got, tc.data) {
				t.Errorf("round-trip mismatch: got %d bytes, want %d", len(got), len(tc.data))
			}
		})
	}
}


// TestRoundTripLSB exercises the LSB byte-order branch of writer/reader.
// Vendored from golang.org/x/image/tiff/lzw alongside MSB; opentile only
// uses MSB (TIFF), but keeping LSB working preserves the upstream contract
// and is a one-test gate against accidentally breaking it.
func TestRoundTripLSB(t *testing.T) {
	cases := [][]byte{
		[]byte("hello LSB world"),
		bytes.Repeat([]byte{0x42}, 512),
		func() []byte {
			out := make([]byte, 2048)
			for i := range out {
				out[i] = byte(i * 3)
			}
			return out
		}(),
	}
	for i, data := range cases {
		var buf bytes.Buffer
		w := NewWriter(&buf, LSB, 8)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("case %d write: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("case %d close: %v", i, err)
		}
		r := NewReader(bytes.NewReader(buf.Bytes()), LSB, 8)
		got, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatalf("case %d read: %v", i, err)
		}
		if !bytes.Equal(got, data) {
			t.Errorf("case %d: round-trip mismatch (got %d bytes, want %d)", i, len(got), len(data))
		}
	}
}
