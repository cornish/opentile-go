package ndpi

import (
	opentile "github.com/tcornish/opentile-go"
)

// tiler is the NDPI implementation of opentile.Tiler.
type tiler struct {
	md         Metadata
	levels     []opentile.Level
	associated []opentile.AssociatedImage
	icc        []byte
}

func (t *tiler) Format() opentile.Format { return opentile.FormatNDPI }

func (t *tiler) Levels() []opentile.Level {
	// Return a defensive copy of the slice header so callers cannot mutate
	// library state. The underlying Level pointers are shared.
	out := make([]opentile.Level, len(t.levels))
	copy(out, t.levels)
	return out
}

func (t *tiler) Associated() []opentile.AssociatedImage { return t.associated }
func (t *tiler) Metadata() opentile.Metadata            { return t.md.Metadata }
func (t *tiler) ICCProfile() []byte                     { return t.icc }
func (t *tiler) Close() error                           { return nil }

func (t *tiler) Level(i int) (opentile.Level, error) {
	if i < 0 || i >= len(t.levels) {
		return nil, opentile.ErrLevelOutOfRange
	}
	return t.levels[i], nil
}
