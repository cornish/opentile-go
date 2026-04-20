package opentile

import "testing"

func TestSize(t *testing.T) {
	s := Size{W: 10, H: 20}
	if s.Area() != 200 {
		t.Fatalf("Area: want 200, got %d", s.Area())
	}
	if s.String() != "10x20" {
		t.Fatalf("String: want 10x20, got %s", s.String())
	}
}

func TestPoint(t *testing.T) {
	p := Point{X: 3, Y: 4}
	if p.String() != "(3,4)" {
		t.Fatalf("String: want (3,4), got %s", p.String())
	}
}

func TestSizeMm(t *testing.T) {
	m := SizeMm{W: 0.5, H: 0.25}
	if m.IsZero() {
		t.Fatal("IsZero: non-zero value reported zero")
	}
	if !(SizeMm{}).IsZero() {
		t.Fatal("IsZero: zero value reported non-zero")
	}
}

func TestRegionContains(t *testing.T) {
	r := Region{Origin: Point{X: 5, Y: 5}, Size: Size{W: 10, H: 10}}
	tests := []struct {
		p    Point
		want bool
	}{
		{Point{X: 5, Y: 5}, true},
		{Point{X: 14, Y: 14}, true},
		{Point{X: 15, Y: 14}, false},
		{Point{X: 4, Y: 5}, false},
	}
	for _, tt := range tests {
		if got := r.Contains(tt.p); got != tt.want {
			t.Errorf("Contains(%v) = %v, want %v", tt.p, got, tt.want)
		}
	}
}
