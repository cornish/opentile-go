package tiff

import (
	"io"

	"golang.org/x/exp/mmap"
)

// MmapFile is an io.ReaderAt + io.Closer over a memory-mapped file.
// Constructed by [OpenMmap]; the Tiler that consumes it owns the
// mapping for its lifetime.
//
// On Linux/macOS the underlying mmap uses PROT_READ + MAP_SHARED
// (lazy paging via the kernel page-fault handler). On Windows it
// uses CreateFileMapping + MapViewOfFile. In both cases ReadAt
// is a userspace memcpy from the mapped region; no syscall per
// call.
//
// Concurrency: ReadAt is safe to call concurrently from any number
// of goroutines (the underlying byte slice is read-only and
// position-less). Close must not race with in-flight ReadAt calls.
//
// Failure mode on truncation: if the underlying file is truncated
// after the mapping is established, ReadAt over the truncated
// region raises SIGBUS in the calling thread. WSI files don't
// get truncated under normal use; if your storage allows it, use
// the explicit pread-backed [OpenFile] path via
// opentile.WithBacking(opentile.BackingPread) (added in v0.9).
type MmapFile struct {
	r *mmap.ReaderAt
}

// OpenMmap memory-maps path read-only and returns a MmapFile. The
// returned MmapFile owns the mapping; Close releases it. Returns
// the underlying os/x error on failure (typically a syscall errno
// for "file does not exist", "permission denied", or a filesystem
// that doesn't support mmap).
func OpenMmap(path string) (*MmapFile, error) {
	r, err := mmap.Open(path)
	if err != nil {
		return nil, err
	}
	return &MmapFile{r: r}, nil
}

// ReadAt implements io.ReaderAt over the mapped region.
func (m *MmapFile) ReadAt(p []byte, off int64) (int, error) {
	return m.r.ReadAt(p, off)
}

// Size returns the length of the mapped region in bytes — the same
// value as the file's size at mmap time.
func (m *MmapFile) Size() int64 {
	return int64(m.r.Len())
}

// Close releases the mapping. Must not race with in-flight ReadAt
// calls on this MmapFile.
func (m *MmapFile) Close() error {
	return m.r.Close()
}

// Verify the type satisfies the consumer-side interfaces.
var (
	_ io.ReaderAt = (*MmapFile)(nil)
	_ io.Closer   = (*MmapFile)(nil)
)
