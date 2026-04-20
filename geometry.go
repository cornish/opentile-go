// Package opentile provides utilities to read tiles from whole-slide imaging
// (WSI) TIFF files. See the repository README for a high-level overview.
package opentile

import "fmt"

// Point is a 2D integer position measured in pixels or tile units.
type Point struct {
	X, Y int
}

func (p Point) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

// Size is a 2D integer extent measured in pixels or tile units.
type Size struct {
	W, H int
}

func (s Size) Area() int      { return s.W * s.H }
func (s Size) String() string { return fmt.Sprintf("%dx%d", s.W, s.H) }

// SizeMm is a 2D extent measured in millimeters. Used for pixel spacing and
// microns-per-pixel conversion (SizeMm scaled by 1000 equals micrometers).
type SizeMm struct {
	W, H float64
}

func (s SizeMm) IsZero() bool { return s.W == 0 && s.H == 0 }

// Region is an axis-aligned rectangle in pixel or tile units.
type Region struct {
	Origin Point
	Size   Size
}

// Contains reports whether p lies inside r (inclusive of origin, exclusive of
// the far edge).
func (r Region) Contains(p Point) bool {
	return p.X >= r.Origin.X && p.X < r.Origin.X+r.Size.W &&
		p.Y >= r.Origin.Y && p.Y < r.Origin.Y+r.Size.H
}
