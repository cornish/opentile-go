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
	tileSize    Size
	hasTileSize bool
	corruptTile CorruptTilePolicy
}

func newConfig(opts []Option) *config {
	c := &config{
		tileSize:    Size{},
		corruptTile: CorruptTileError,
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

// Config is an opaque, read-only view of the configuration passed to a
// FormatFactory. Format packages import opentile.Config rather than the
// unexported config struct.
type Config struct {
	c *config
}

// TileSize returns the requested output tile size and whether the caller set
// one. ok=false means "use format default"; callers must not treat the zero
// Size as equivalent to "default" because (Size{}, true) is distinct from
// (Size{}, false) — the former asserts an explicit 0x0 (which format packages
// should reject as malformed input).
func (c *Config) TileSize() (Size, bool) { return c.c.tileSize, c.c.hasTileSize }

// CorruptTilePolicy returns the configured policy.
func (c *Config) CorruptTilePolicy() CorruptTilePolicy { return c.c.corruptTile }

// NewTestConfig constructs a Config for use in tests. It is not intended for
// production callers, which should go through opentile.Open. A non-zero
// tileSize is treated as explicitly set (TileSize ok=true); a zero Size is
// treated as "use format default" (TileSize ok=false).
func NewTestConfig(tileSize Size, policy CorruptTilePolicy) *Config {
	has := tileSize.W != 0 || tileSize.H != 0
	return &Config{c: &config{tileSize: tileSize, hasTileSize: has, corruptTile: policy}}
}
