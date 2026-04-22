package tiff

import (
	"bytes"
	"errors"
	"testing"
)

func TestParseHeader(t *testing.T) {
	tests := []struct {
		name       string
		bytes      []byte
		wantLE     bool
		wantBig    bool
		wantOffset uint64
		wantErr    error
	}{
		{
			name:       "little-endian classic",
			bytes:      []byte{'I', 'I', 42, 0, 0x08, 0, 0, 0},
			wantLE:     true,
			wantBig:    false,
			wantOffset: 8,
		},
		{
			name:       "big-endian classic",
			bytes:      []byte{'M', 'M', 0, 42, 0, 0, 0, 0x10},
			wantLE:     false,
			wantBig:    false,
			wantOffset: 16,
		},
		{
			name:    "bad byte order",
			bytes:   []byte{'X', 'Y', 42, 0, 0, 0, 0, 0},
			wantErr: ErrInvalidTIFF,
		},
		{
			name:    "bad magic",
			bytes:   []byte{'I', 'I', 99, 0, 0, 0, 0, 0},
			wantErr: ErrInvalidTIFF,
		},
		{
			name:       "bigtiff",
			bytes:      []byte{'I', 'I', 43, 0, 8, 0, 0, 0, 0x20, 0, 0, 0, 0, 0, 0, 0},
			wantLE:     true,
			wantBig:    true,
			wantOffset: 0x20,
		},
		{
			name:    "short",
			bytes:   []byte{'I', 'I'},
			wantErr: ErrInvalidTIFF,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := parseHeader(bytes.NewReader(tt.bytes))
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err: got %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if h.littleEndian != tt.wantLE {
				t.Errorf("littleEndian: got %v, want %v", h.littleEndian, tt.wantLE)
			}
			if h.bigTIFF != tt.wantBig {
				t.Errorf("bigTIFF: got %v, want %v", h.bigTIFF, tt.wantBig)
			}
			if h.firstIFD != tt.wantOffset {
				t.Errorf("firstIFD: got %d, want %d", h.firstIFD, tt.wantOffset)
			}
		})
	}
}
