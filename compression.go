package opentile

import "fmt"

// Compression identifies the bitstream format of a tile as stored in a TIFF.
//
// opentile-go returns tile bytes in the compression format of the source TIFF
// without decoding them. Consumers that need decoded pixels should pass the
// bytes to a codec appropriate for the reported compression.
type Compression uint8

const (
	CompressionNone Compression = iota
	CompressionJPEG
	CompressionJP2K
)

func (c Compression) String() string {
	switch c {
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
