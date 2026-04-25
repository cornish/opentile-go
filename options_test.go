package opentile

import "testing"

func TestDefaultConfig(t *testing.T) {
	c := newConfig(nil)
	if c.tileSize.W != 0 || c.tileSize.H != 0 {
		t.Errorf("tileSize default: got %v, want 0,0", c.tileSize)
	}
	if c.hasTileSize {
		t.Error("hasTileSize default: expected false")
	}
	if c.corruptTile != CorruptTileError {
		t.Errorf("corruptTile default: got %v, want CorruptTileError", c.corruptTile)
	}
}

func TestWithTileSize(t *testing.T) {
	c := newConfig([]Option{WithTileSize(512, 256)})
	if c.tileSize.W != 512 || c.tileSize.H != 256 {
		t.Errorf("tileSize: got %v, want 512x256", c.tileSize)
	}
	if !c.hasTileSize {
		t.Error("hasTileSize: expected true after WithTileSize")
	}
}

func TestWithCorruptTilePolicy(t *testing.T) {
	c := newConfig([]Option{WithCorruptTilePolicy(CorruptTileError)})
	if c.corruptTile != CorruptTileError {
		t.Errorf("policy: got %v", c.corruptTile)
	}
}

func TestConfigTileSizeAccessor(t *testing.T) {
	// Default: ok=false
	c := &Config{c: newConfig(nil)}
	if _, ok := c.TileSize(); ok {
		t.Error("default TileSize: expected ok=false")
	}
	// With option set: ok=true, Size preserved.
	c = &Config{c: newConfig([]Option{WithTileSize(512, 256)})}
	sz, ok := c.TileSize()
	if !ok {
		t.Error("TileSize ok after WithTileSize: expected true")
	}
	if sz.W != 512 || sz.H != 256 {
		t.Errorf("TileSize value: got %v, want 512x256", sz)
	}
	// NewTestConfig with non-zero size: ok=true.
	c = NewTestConfig(Size{W: 128, H: 64}, CorruptTileError)
	if _, ok := c.TileSize(); !ok {
		t.Error("NewTestConfig with non-zero tileSize: expected ok=true")
	}
	// NewTestConfig with zero size: ok=false.
	c = NewTestConfig(Size{}, CorruptTileError)
	if _, ok := c.TileSize(); ok {
		t.Error("NewTestConfig with zero tileSize: expected ok=false")
	}
}

func TestConfigTileSizeExplicitZero(t *testing.T) {
	c := &Config{c: newConfig([]Option{WithTileSize(0, 0)})}
	sz, ok := c.TileSize()
	if !ok {
		t.Fatal("explicit WithTileSize(0,0): expected ok=true")
	}
	if sz != (Size{}) {
		t.Errorf("explicit WithTileSize(0,0): got %v, want zero Size", sz)
	}
}

func TestConfigTileSizeUnsetVsExplicitZero(t *testing.T) {
	cUnset := &Config{c: newConfig(nil)}
	cExplicit := &Config{c: newConfig([]Option{WithTileSize(0, 0)})}
	_, okUnset := cUnset.TileSize()
	_, okExplicit := cExplicit.TileSize()
	if okUnset {
		t.Error("unset config: expected ok=false")
	}
	if !okExplicit {
		t.Error("explicit zero: expected ok=true")
	}
}
