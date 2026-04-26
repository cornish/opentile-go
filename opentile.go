package opentile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/tcornish/opentile-go/internal/tiff"
)

// FormatFactory is implemented by format packages to register themselves with
// the top-level opentile package. Factories are queried in registration order;
// the first factory whose Supports() returns true is used.
type FormatFactory interface {
	Format() Format
	Supports(file *tiff.File) bool
	Open(file *tiff.File, cfg *Config) (Tiler, error)
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

// Open parses r as a WSI TIFF and returns a Tiler for the matching format.
// size is the total file size in bytes.
func Open(r io.ReaderAt, size int64, opts ...Option) (Tiler, error) {
	cfg := newConfig(opts)
	file, err := tiff.Open(r, size)
	if err != nil {
		if errors.Is(err, tiff.ErrInvalidTIFF) {
			return nil, fmt.Errorf("%w: %v", ErrInvalidTIFF, err)
		}
		return nil, err
	}
	registryMu.RLock()
	factories := append([]FormatFactory(nil), registry...)
	registryMu.RUnlock()

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
