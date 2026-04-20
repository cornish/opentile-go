package opentile

import "fmt"

// Compression identifies the bitstream format of a tile as stored in a TIFF.
//
// opentile-go returns tile bytes in the compression format of the source TIFF
// without decoding them. Consumers that need decoded pixels should pass the
// bytes to a codec appropriate for the reported compression.
//
// The zero value is CompressionUnknown: a forgotten-to-initialize field
// surfaces loudly rather than masquerading as a known compression.
type Compression uint8

const (
	CompressionUnknown Compression = iota // zero value; unset or unrecognized
	CompressionNone
	CompressionJPEG
	CompressionJP2K
)

func (c Compression) String() string {
	switch c {
	case CompressionUnknown:
		return "unknown"
	case CompressionNone:
		return "none"
	case CompressionJPEG:
		return "jpeg"
	case CompressionJP2K:
		return "jp2k"
	default:
		return fmt.Sprintf("unknown(%d)", uint8(c))
	}
}
