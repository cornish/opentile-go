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
	Levels() []Level
	Level(i int) (Level, error)
	Associated() []AssociatedImage // v0.1: always returns nil; associated images land in v0.3
	Metadata() Metadata
	ICCProfile() []byte
	Close() error
}
