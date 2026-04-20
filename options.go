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
	return func(c *config) { c.tileSize = Size{W: w, H: h} }
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

// TileSize returns the requested output tile size. A zero Size means "format
// default".
func (c *Config) TileSize() Size { return c.c.tileSize }

// CorruptTilePolicy returns the configured policy.
func (c *Config) CorruptTilePolicy() CorruptTilePolicy { return c.c.corruptTile }

// NewTestConfig constructs a Config for use in tests. It is not intended for
// production callers, which should go through opentile.Open.
func NewTestConfig(tileSize Size, policy CorruptTilePolicy) *Config {
	return &Config{c: &config{tileSize: tileSize, corruptTile: policy}}
}
