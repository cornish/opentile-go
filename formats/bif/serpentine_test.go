package bif

import "testing"

// TestSerpentineRoundTrip: imageToSerpentine and serpentineToImage
// are mutual inverses on every (col, row) of a 24×21 grid (the
// dimensions of Ventana-1's level 0).
func TestSerpentineRoundTrip(t *testing.T) {
	const cols, rows = 24, 21
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			idx := imageToSerpentine(c, r, cols, rows)
			if idx < 0 {
				t.Fatalf("imageToSerpentine(%d,%d): got -1, expected valid index", c, r)
			}
			gotCol, gotRow := serpentineToImage(idx, cols, rows)
			if gotCol != c || gotRow != r {
				t.Errorf("round-trip (%d,%d) -> idx=%d -> (%d,%d): mismatch", c, r, idx, gotCol, gotRow)
			}
		}
	}
}

// TestSerpentineCornersAndAnchors verifies hand-computed indices
// for a 24×21 grid (Ventana-1 level 0 dimensions). Per the
// whitepaper Figure 2:
//   - Tile 0 is at bottom-left of the AOI = image (col=0, row=20).
//   - Tile 23 is at bottom-right of the AOI = image (col=23, row=20).
//   - Tile 24 starts stage row 1 (right-to-left) = image (col=23, row=19).
//   - Tile 480 starts stage row 20 (even, left-to-right) at image
//     (col=0, row=0) — the image-top-left corner with rows=21.
func TestSerpentineCornersAndAnchors(t *testing.T) {
	const cols, rows = 24, 21
	cases := []struct {
		name     string
		col, row int
		wantIdx  int
	}{
		{"bottom-left (first tile)", 0, 20, 0},
		{"bottom-right (end of stage row 0)", 23, 20, 23},
		{"start stage row 1 (right-to-left)", 23, 19, 24},
		{"end stage row 1", 0, 19, 47},
		{"start stage row 2 (back to L→R)", 0, 18, 48},
		{"top-left (col=0, row=0; stage row 20 even, stage col 0)", 0, 0, 20 * cols},
		{"top-right (col=23, row=0; stage row 20 even, stage col 23)", 23, 0, 20*cols + 23},
	}
	for _, tc := range cases {
		got := imageToSerpentine(tc.col, tc.row, cols, rows)
		if got != tc.wantIdx {
			t.Errorf("%s: imageToSerpentine(%d,%d,%d,%d) = %d, want %d", tc.name, tc.col, tc.row, cols, rows, got, tc.wantIdx)
		}
	}
}

// TestSerpentineOutOfBounds: imageToSerpentine returns -1 for
// negative or beyond-grid (col, row); serpentineToImage returns
// (-1, -1) for negative or beyond-count idx.
func TestSerpentineOutOfBounds(t *testing.T) {
	const cols, rows = 4, 3
	bad := [][3]int{{-1, 0, 0}, {0, -1, 0}, {4, 0, 0}, {0, 3, 0}, {99, 99, 0}}
	for _, c := range bad {
		if got := imageToSerpentine(c[0], c[1], cols, rows); got != -1 {
			t.Errorf("imageToSerpentine(%d,%d,%d,%d): got %d, want -1", c[0], c[1], cols, rows, got)
		}
	}
	for _, idx := range []int{-1, 12, 99} {
		if c, r := serpentineToImage(idx, cols, rows); c != -1 || r != -1 {
			t.Errorf("serpentineToImage(%d,%d,%d): got (%d,%d), want (-1,-1)", idx, cols, rows, c, r)
		}
	}
}

// TestSerpentineSingleRowGrid: edge case — a single-row grid (rows=1).
// Stage row 0 is even → left-to-right; idx == col.
func TestSerpentineSingleRowGrid(t *testing.T) {
	const cols, rows = 5, 1
	for c := 0; c < cols; c++ {
		idx := imageToSerpentine(c, 0, cols, rows)
		if idx != c {
			t.Errorf("imageToSerpentine(%d,0,%d,1) = %d, want %d", c, cols, idx, c)
		}
	}
}

// TestSerpentineSingleColGrid: edge case — a single-column grid
// (cols=1). Stage rows alternate parity but every row has only one
// tile so the col flip is a no-op (cols-1 - 0 = 0). idx counts up
// from bottom.
func TestSerpentineSingleColGrid(t *testing.T) {
	const cols, rows = 1, 5
	for r := 0; r < rows; r++ {
		idx := imageToSerpentine(0, r, cols, rows)
		want := rows - 1 - r
		if idx != want {
			t.Errorf("imageToSerpentine(0,%d,1,%d) = %d, want %d", r, rows, idx, want)
		}
	}
}
