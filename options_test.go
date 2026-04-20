package opentile

import "testing"

func TestDefaultConfig(t *testing.T) {
	c := newConfig(nil)
	if c.tileSize.W != 0 || c.tileSize.H != 0 {
		t.Errorf("tileSize default: got %v, want 0,0", c.tileSize)
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
}

func TestWithCorruptTilePolicy(t *testing.T) {
	c := newConfig([]Option{WithCorruptTilePolicy(CorruptTileError)})
	if c.corruptTile != CorruptTileError {
		t.Errorf("policy: got %v", c.corruptTile)
	}
}
