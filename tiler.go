package opentile

// Format identifies the source file format.
type Format string

const (
	FormatSVS     Format = "svs"
	FormatNDPI    Format = "ndpi"
	FormatPhilips Format = "philips"
	FormatOME     Format = "ome"
	FormatBIF     Format = "bif"
	FormatIFE     Format = "ife"
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

	// WarmLevel pre-warms the page cache for level i by touching one
	// byte per OS page covering this level's tile-data ranges. Under
	// mmap-backed [OpenFile] (the v0.9 default), this forces the
	// kernel to populate the page cache lazily on first call —
	// subsequent [Level.Tile] / [Level.TileInto] reads on level i hit
	// RAM at memory-bandwidth speed regardless of access pattern.
	//
	// Under [BackingPread], WarmLevel does effectively the same work
	// via a pread(1) per page. Slower, but the warm-up effect
	// (kernel page cache population) is the same.
	//
	// Returns [ErrLevelOutOfRange] if i is out of bounds. Returns
	// the first non-EOF read error encountered while touching pages
	// (typically I/O errors on the underlying file). nil on success.
	//
	// Best-effort: callers that want to ignore errors (it's a hint,
	// after all) can discard the result. Concurrent calls on
	// different levels are safe; concurrent calls on the same level
	// are safe but redundant.
	//
	// Added in v0.9 alongside the mmap-default [OpenFile] change.
	WarmLevel(i int) error

	Close() error
}
