package opentile

// CorruptTilePolicy controls how corrupt-edge tiles (currently: Aperio SVS) are
// reported. v0.1 supports only CorruptTileError.
type CorruptTilePolicy uint8

const (
	CorruptTileError CorruptTilePolicy = iota // return ErrCorruptTile (default, v0.1)
	CorruptTileBlank                          // v0.3: return a typed blank tile
	CorruptTileFix                            // v1.0: reconstruct from parent level
)

// Option mutates the opentile configuration before Open returns a Tiler.
type Option func(*config)

// config is the aggregate of all Option values applied at Open time.
type config struct {
	tileSize       Size
	hasTileSize    bool
	corruptTile    CorruptTilePolicy
	ndpiSynthLabel bool // default true
}

func newConfig(opts []Option) *config {
	c := &config{
		tileSize:       Size{},
		corruptTile:    CorruptTileError,
		ndpiSynthLabel: true, // v0.2 behavior; opt-out via WithNDPISynthesizedLabel(false)
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// WithTileSize requests output tile dimensions in pixels. If unset, the format
// default is used (SVS: native tile size from the TIFF). Required for formats
// that have no native rectangular tiles (NDPI, v0.2+).
func WithTileSize(w, h int) Option {
	return func(c *config) {
		c.tileSize = Size{W: w, H: h}
		c.hasTileSize = true
	}
}

// WithCorruptTilePolicy sets the behavior for corrupt-edge tiles. v0.1 supports
// only CorruptTileError.
func WithCorruptTilePolicy(p CorruptTilePolicy) Option {
	return func(c *config) { c.corruptTile = p }
}

// WithNDPISynthesizedLabel controls whether NDPI Tiler.Associated() includes
// a synthesized "label" image, which Go produces by cropping the left 30%
// of the overview page. Python opentile 0.20.0 does not expose NDPI labels;
// this is a Go-side extension. Default: true (matches v0.2 behavior).
func WithNDPISynthesizedLabel(enable bool) Option {
	return func(c *config) {
		c.ndpiSynthLabel = enable
	}
}

// Config is an opaque, read-only view of the configuration passed to a
// FormatFactory. Format packages import opentile.Config rather than the
// unexported config struct.
type Config struct {
	c *config
}

// TileSize returns the requested output tile size and whether the caller
// set one.
//
//   - (Size{}, false): caller did not pass WithTileSize. Format packages
//     should use their format default (e.g. SVS reads the native tile size
//     from the TIFF; NDPI uses 512).
//   - (Size{}, true): caller explicitly passed WithTileSize(0, 0). Format
//     packages MUST reject this as malformed input. The zero Size is
//     distinct from "unset" because the API contract is that an explicit
//     option overrides the default.
//   - (non-zero, true): caller's requested tile size; format honors it
//     (NDPI may snap to a stripe-multiple, SVS rejects when it doesn't
//     match the native tile dimensions).
func (c *Config) TileSize() (Size, bool) { return c.c.tileSize, c.c.hasTileSize }

// CorruptTilePolicy returns the configured policy.
func (c *Config) CorruptTilePolicy() CorruptTilePolicy { return c.c.corruptTile }

// NDPISynthesizedLabel reports whether NDPI Tiler.Associated() should
// include a synthesized label cropped from the overview. Default true.
func (c *Config) NDPISynthesizedLabel() bool { return c.c.ndpiSynthLabel }

// NewTestConfig constructs a Config for use in tests.
//
// Deprecated: use opentile/opentiletest.NewConfig. This wrapper remains
// for one release to keep external callers compiling; it will be removed
// in v0.4.
func NewTestConfig(tileSize Size, policy CorruptTilePolicy) *Config {
	has := tileSize.W != 0 || tileSize.H != 0
	return &Config{c: &config{tileSize: tileSize, hasTileSize: has, corruptTile: policy}}
}
