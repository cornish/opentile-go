package opentile

// Format identifies the source file format.
type Format string

const (
	FormatSVS  Format = "svs"
	FormatNDPI Format = "ndpi" // defined for future use; not implemented in v0.1
)

// Tiler is the top-level handle returned by Open. All accessors are
// immutable and safe for concurrent use; Close() must not race with in-flight
// tile reads.
type Tiler interface {
	Format() Format
	Levels() []Level
	Level(i int) (Level, error)
	Associated() []AssociatedImage
	Metadata() Metadata
	ICCProfile() []byte
	Close() error
}
