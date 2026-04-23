package tiff

import (
	"errors"
	"fmt"
	"io"
)

// ReadAtFull is io.ReaderAt.ReadAt wrapped to tolerate the "full buffer + io.EOF"
// case that ReadAt is permitted to return per the stdlib contract. It returns
// nil iff the full buffer was filled, regardless of whether err was nil or EOF.
// Any short read or other error is surfaced to the caller.
//
// Per io.ReaderAt's documented contract: "If the n = len(p) bytes returned by
// ReadAt are at the end of the input source, ReadAt may return either
// err == EOF or err == nil." A naive err != nil check therefore spuriously
// fails on backends (e.g. bytes.Reader) that return the EOF flavor whenever a
// read lands exactly at end-of-input — which is a legitimate layout for the
// last strip or tile of a TIFF.
func ReadAtFull(r io.ReaderAt, buf []byte, off int64) error {
	n, err := r.ReadAt(buf, off)
	if err != nil && !(errors.Is(err, io.EOF) && n == len(buf)) {
		return err
	}
	if n != len(buf) {
		return fmt.Errorf("short read at %d: got %d, want %d", off, n, len(buf))
	}
	return nil
}
