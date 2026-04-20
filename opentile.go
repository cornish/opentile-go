package opentile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/tcornish/opentile-go/internal/tiff"
)

// FormatFactory is implemented by format packages to register themselves with
// the top-level opentile package. Factories are queried in registration order;
// the first factory whose Supports() returns true is used.
type FormatFactory interface {
	Format() Format
	Supports(file *tiff.File) bool
	Open(file *tiff.File, cfg *config) (Tiler, error)
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

// resetRegistry is for tests only.
func resetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = nil
}

// Open parses r as a WSI TIFF and returns a Tiler for the matching format.
// size is the total file size in bytes.
func Open(r io.ReaderAt, size int64, opts ...Option) (Tiler, error) {
	cfg := newConfig(opts)
	file, err := tiff.Open(r)
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
			return f.Open(file, cfg)
		}
	}
	return nil, ErrUnsupportedFormat
}

// OpenFile opens path for reading and delegates to Open. The returned Tiler
// owns the file handle; Close closes it.
func OpenFile(path string, opts ...Option) (Tiler, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	t, err := Open(f, info.Size(), opts...)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &fileCloser{Tiler: t, f: f}, nil
}

// fileCloser overrides Close to also close the underlying file.
type fileCloser struct {
	Tiler
	f *os.File
}

func (fc *fileCloser) Close() error {
	err1 := fc.Tiler.Close()
	err2 := fc.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
}
