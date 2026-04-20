package opentile

import "testing"

func TestCompressionString(t *testing.T) {
	tests := []struct {
		c    Compression
		want string
	}{
		{CompressionNone, "none"},
		{CompressionJPEG, "jpeg"},
		{CompressionJP2K, "jp2k"},
		{Compression(99), "unknown(99)"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("Compression(%d).String() = %q, want %q", tt.c, got, tt.want)
		}
	}
}
