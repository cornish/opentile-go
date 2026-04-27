package opentile

// Format identifies the source file format.
type Format string

const (
	FormatSVS     Format = "svs"
	FormatNDPI    Format = "ndpi"
	FormatPhilips Format = "philips"
)

// Tiler is the top-level handle returned by Open. All accessors are
// immutable and safe for concurrent use; Close() must not race with in-flight
// tile reads.
type Tiler interface {
	Format() Format
	// Images returns the main pyramids carried by this file. Always
	// returns at least one Image; multi-image OME TIFF files expose
	// multiple. Index 0 corresponds to the legacy Levels() / Level(i)
	// shortcut accessors.
	//
	// Added in v0.6. Single-image formats (SVS, NDPI, Philips) return a
	// one-element slice wrapping their existing pyramid.
	Images() []Image
	// Levels is a shortcut for Images()[0].Levels(). Preserved from
	// pre-v0.6 callers; behaves identically on single-image formats.
	Levels() []Level
	// Level is a shortcut for Images()[0].Level(i).
	Level(i int) (Level, error)
	Associated() []AssociatedImage
	Metadata() Metadata
	ICCProfile() []byte
	Close() error
}
