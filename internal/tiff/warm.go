package tiff

import (
	"io"
	"os"
)

// pageSize is os.Getpagesize() captured once. Calling it repeatedly
// is cheap but pinning it removes the dependency from the hot path
// in TouchPages.
var pageSize = int64(os.Getpagesize())

// TouchPages reads one byte from each OS-page-aligned position in
// [offset, offset+length). On an mmap-backed io.ReaderAt this forces
// the kernel page-fault handler to bring those pages into the page
// cache, eliminating cold-cache latency on subsequent reads. On a
// pread-backed io.ReaderAt it has the same warm-up effect via a
// pread(1) per page (slower; documented as best-effort).
//
// Returns the first non-EOF read error (nil on success). EOF on the
// last page is silently swallowed because read-past-EOF on a tile
// range that straddles the file's end isn't actually an error —
// kernel readahead has handled what it can.
//
// Does no work for length <= 0. Safe to call concurrently from many
// goroutines (each issues its own ReadAt; no shared state).
func TouchPages(r io.ReaderAt, offset, length int64) error {
	if length <= 0 {
		return nil
	}
	var buf [1]byte
	end := offset + length
	// Round offset down to its page boundary.
	start := offset &^ (pageSize - 1)
	for p := start; p < end; p += pageSize {
		if _, err := r.ReadAt(buf[:], p); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}
