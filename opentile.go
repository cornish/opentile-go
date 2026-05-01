package opentile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/cornish/opentile-go/internal/tiff"
)

// FormatFactory is implemented by format packages to register themselves with
// the top-level opentile package. Factories are queried in registration order;
// the first factory whose Supports() (TIFF path) or SupportsRaw() (non-TIFF
// path) returns true is used.
//
// SupportsRaw + OpenRaw provide a non-TIFF dispatch path. Open's reader
// queries SupportsRaw first; if any factory takes the byte-level dispatch,
// the input is never handed to tiff.Open. Format packages whose files are
// classic/BigTIFF should embed [RawUnsupported] for the zero-value default
// impls (SupportsRaw → false, OpenRaw → ErrUnsupportedFormat). The first
// non-TIFF format to use this path is Iris IFE in v0.8.
type FormatFactory interface {
	Format() Format
	SupportsRaw(r io.ReaderAt, size int64) bool
	OpenRaw(r io.ReaderAt, size int64, cfg *Config) (Tiler, error)
	Supports(file *tiff.File) bool
	Open(file *tiff.File, cfg *Config) (Tiler, error)
}

// RawUnsupported is the zero-impl base for [FormatFactory.SupportsRaw] +
// [FormatFactory.OpenRaw]. Format packages whose files are classic or
// BigTIFF embed this struct in their Factory type to inherit the
// "doesn't take the non-TIFF dispatch path" defaults. Non-TIFF format
// packages (Iris IFE) override both methods on their own Factory.
type RawUnsupported struct{}

// SupportsRaw reports false: this format doesn't recognize raw byte streams,
// so the dispatch loop continues to the TIFF path.
func (RawUnsupported) SupportsRaw(io.ReaderAt, int64) bool { return false }

// OpenRaw returns ErrUnsupportedFormat; the dispatch loop never reaches
// this method on a TIFF-only factory because SupportsRaw returns false
// first, but the explicit error keeps callers honest if they bypass Open.
func (RawUnsupported) OpenRaw(io.ReaderAt, int64, *Config) (Tiler, error) {
	return nil, ErrUnsupportedFormat
}

var (
	registryMu sync.RWMutex
	registry   []FormatFactory
)

// Register adds a FormatFactory to the registry. It is safe to call concurrently
// but typically called from init or a main-package setup function.
func Register(f FormatFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, f)
}

// Formats returns the format identifiers that have been registered via
// Register, sorted lexicographically. Useful for diagnostics and for
// callers that want to enumerate compiled-in formats without importing
// each format package directly.
func Formats() []Format {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Format, 0, len(registry))
	for _, f := range registry {
		out = append(out, f.Format())
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}

// Open parses r as a WSI file and returns a Tiler for the matching format.
// size is the total file size in bytes.
//
// Dispatch order: each registered factory's SupportsRaw is queried first
// against the raw byte stream; if any factory takes it, the input is
// never handed to tiff.Open. Otherwise, r is parsed as TIFF / BigTIFF
// and each factory's Supports is queried against the parsed *tiff.File.
// The first non-TIFF format using the SupportsRaw path is Iris IFE in
// v0.8.
func Open(r io.ReaderAt, size int64, opts ...Option) (Tiler, error) {
	cfg := newConfig(opts)
	registryMu.RLock()
	factories := append([]FormatFactory(nil), registry...)
	registryMu.RUnlock()

	for _, f := range factories {
		if f.SupportsRaw(r, size) {
			return f.OpenRaw(r, size, &Config{c: cfg})
		}
	}

	file, err := tiff.Open(r, size)
	if err != nil {
		if errors.Is(err, tiff.ErrInvalidTIFF) {
			return nil, fmt.Errorf("%w: %v", ErrInvalidTIFF, err)
		}
		return nil, err
	}

	for _, f := range factories {
		if f.Supports(file) {
			return f.Open(file, &Config{c: cfg})
		}
	}
	return nil, ErrUnsupportedFormat
}

// OpenFile opens path for reading and delegates to Open. The returned Tiler
// owns the file handle; Close closes it.
func OpenFile(path string, opts ...Option) (Tiler, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opentile: open %q: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("opentile: stat %q: %w", path, err)
	}
	t, err := Open(f, info.Size(), opts...)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("opentile: %s: %w", path, err)
	}
	return &fileCloser{Tiler: t, f: f}, nil
}

// fileCloser overrides Close to also close the underlying file.
type fileCloser struct {
	Tiler
	f *os.File
}

func (fc *fileCloser) Close() error {
	return errors.Join(fc.Tiler.Close(), fc.f.Close())
}

// UnwrapTiler exposes the wrapped Tiler so format packages can reach their
// concrete implementation through type assertion via svs.MetadataOf (and
// equivalent per-format accessors) when the consumer obtained the Tiler
// through OpenFile rather than Open.
func (fc *fileCloser) UnwrapTiler() Tiler { return fc.Tiler }
