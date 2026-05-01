package tiff

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMmap_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob")
	body := []byte("the quick brown fox jumps over the lazy dog")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m, err := OpenMmap(path)
	if err != nil {
		t.Fatalf("OpenMmap: %v", err)
	}
	defer m.Close()

	if got, want := m.Size(), int64(len(body)); got != want {
		t.Errorf("Size = %d, want %d", got, want)
	}

	// Full read.
	got := make([]byte, len(body))
	n, err := m.ReadAt(got, 0)
	if err != nil {
		t.Fatalf("ReadAt full: %v", err)
	}
	if n != len(body) {
		t.Errorf("ReadAt n = %d, want %d", n, len(body))
	}
	if string(got) != string(body) {
		t.Errorf("ReadAt body = %q, want %q", got, body)
	}

	// Partial read at offset.
	got2 := make([]byte, 5)
	if _, err := m.ReadAt(got2, 4); err != nil {
		t.Fatalf("ReadAt partial: %v", err)
	}
	if string(got2) != "quick" {
		t.Errorf("ReadAt partial body = %q, want %q", got2, "quick")
	}
}

func TestOpenMmap_NonexistentFile(t *testing.T) {
	_, err := OpenMmap(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("OpenMmap on missing file: got nil error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("got %v, want errors.Is(os.ErrNotExist)", err)
	}
}

func TestOpenMmap_PastEOF(t *testing.T) {
	// ReadAt past the end of the mapping returns io.EOF (and possibly
	// fewer bytes than requested). The exact semantics match
	// io.ReaderAt's contract — pin them for our consumers.
	dir := t.TempDir()
	path := filepath.Join(dir, "tiny")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := OpenMmap(path)
	if err != nil {
		t.Fatalf("OpenMmap: %v", err)
	}
	defer m.Close()

	// Read at the EOF boundary — EOF, zero bytes.
	buf := make([]byte, 4)
	n, err := m.ReadAt(buf, int64(m.Size()))
	if !errors.Is(err, io.EOF) {
		t.Errorf("ReadAt at EOF: err = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("ReadAt at EOF: n = %d, want 0", n)
	}

	// Read straddling EOF — partial read + EOF.
	buf = make([]byte, 8)
	n, err = m.ReadAt(buf, 2)
	if !errors.Is(err, io.EOF) {
		t.Errorf("ReadAt straddling EOF: err = %v, want io.EOF", err)
	}
	if n != 3 {
		t.Errorf("ReadAt straddling EOF: n = %d, want 3", n)
	}
	if string(buf[:n]) != "345" {
		t.Errorf("ReadAt straddling EOF: body = %q, want \"345\"", buf[:n])
	}
}

func TestOpenMmap_Concurrent(t *testing.T) {
	// Pin the contract that ReadAt is concurrent-safe (no lock
	// required at the wrapper level — x/exp/mmap's underlying byte
	// slice is read-only and shared).
	dir := t.TempDir()
	path := filepath.Join(dir, "concurrent")
	body := make([]byte, 4096)
	for i := range body {
		body[i] = byte(i & 0xFF)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := OpenMmap(path)
	if err != nil {
		t.Fatalf("OpenMmap: %v", err)
	}
	defer m.Close()

	const goroutines = 16
	const iters = 1000
	done := make(chan error, goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			buf := make([]byte, 16)
			for i := 0; i < iters; i++ {
				off := int64((i * 7) % (len(body) - 16))
				if _, err := m.ReadAt(buf, off); err != nil {
					done <- err
					return
				}
				if buf[0] != byte(off) {
					done <- errors.New("byte mismatch")
					return
				}
			}
			done <- nil
		}()
	}
	for g := 0; g < goroutines; g++ {
		if err := <-done; err != nil {
			t.Errorf("goroutine err: %v", err)
		}
	}
}
