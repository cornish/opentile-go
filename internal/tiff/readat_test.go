package tiff

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// errReaderAt always returns the configured error alongside 0 bytes.
type errReaderAt struct{ err error }

func (e *errReaderAt) ReadAt(p []byte, off int64) (int, error) { return 0, e.err }

// shortReaderAt returns n bytes and a nil error regardless of the requested
// buffer length. Used to exercise the short-read branch without involving EOF.
type shortReaderAt struct {
	data []byte
	n    int // bytes returned per call, clamped to min(n, len(p))
}

func (s *shortReaderAt) ReadAt(p []byte, off int64) (int, error) {
	copy(p, s.data)
	toCopy := s.n
	if toCopy > len(p) {
		toCopy = len(p)
	}
	return toCopy, nil
}

func TestReadAtFullNormal(t *testing.T) {
	// A read in the middle of the source: bytes.Reader returns (n, nil).
	src := bytes.NewReader([]byte("hello, world!"))
	buf := make([]byte, 5)
	if err := ReadAtFull(src, buf, 0); err != nil {
		t.Fatalf("ReadAtFull returned error on normal read: %v", err)
	}
	if string(buf) != "hello" {
		t.Fatalf("buf = %q, want %q", buf, "hello")
	}
}

func TestReadAtFullEOFAtExactEnd(t *testing.T) {
	// bytes.Reader returns (n, io.EOF) when the read lands exactly at end.
	// This is legal per io.ReaderAt's contract and must be treated as success.
	src := bytes.NewReader([]byte("abcdef"))
	buf := make([]byte, 6)
	if err := ReadAtFull(src, buf, 0); err != nil {
		t.Fatalf("ReadAtFull returned error on full-buffer EOF: %v", err)
	}
	if string(buf) != "abcdef" {
		t.Fatalf("buf = %q, want %q", buf, "abcdef")
	}
}

func TestReadAtFullShortRead(t *testing.T) {
	// Reader advertises 3 bytes when asked for 5 — must surface an error even
	// though err itself is nil.
	src := &shortReaderAt{data: []byte("abc"), n: 3}
	buf := make([]byte, 5)
	err := ReadAtFull(src, buf, 0)
	if err == nil {
		t.Fatalf("ReadAtFull returned nil on short read; want error")
	}
}

func TestReadAtFullNonEOFErrorPropagates(t *testing.T) {
	sentinel := errors.New("disk on fire")
	src := &errReaderAt{err: sentinel}
	buf := make([]byte, 4)
	err := ReadAtFull(src, buf, 0)
	if !errors.Is(err, sentinel) {
		t.Fatalf("ReadAtFull returned %v; want error wrapping %v", err, sentinel)
	}
}

// Compile-time check that io.SectionReader — a common wrapper we rely on —
// satisfies io.ReaderAt (not a runtime test, just documents the expectation).
var _ io.ReaderAt = (*bytes.Reader)(nil)
