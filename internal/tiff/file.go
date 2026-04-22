package tiff

import (
	"encoding/binary"
	"io"
)

// File is a parsed TIFF file: a list of pages in IFD order, plus the reader
// and byte order needed to decode tag values and read tile payloads.
type File struct {
	r       io.ReaderAt
	size    int64
	reader  *byteReader
	pages   []*Page
	bigTIFF bool // true when file uses BigTIFF (magic 43, 8-byte offsets)
}

// Open parses the header and every IFD in r, producing a File ready for use by
// format packages. Open does not read tile payloads. The caller retains
// ownership of r; File does not close it (the io.ReaderAt contract does not
// include Close). size is the total readable size of r in bytes and is stored
// for future offset-bounds validation.
func Open(r io.ReaderAt, size int64) (*File, error) {
	h, err := parseHeader(r)
	if err != nil {
		return nil, err
	}
	br := newByteReader(r, h.littleEndian)
	ifds, err := walkIFDs(br, int64(h.firstIFD), h.bigTIFF)
	if err != nil {
		return nil, err
	}
	pages := make([]*Page, 0, len(ifds))
	for _, i := range ifds {
		pages = append(pages, newPage(i, br))
	}
	return &File{r: r, size: size, reader: br, pages: pages, bigTIFF: h.bigTIFF}, nil
}

// Pages returns the pages in IFD order. The slice is owned by File; do not mutate.
func (f *File) Pages() []*Page { return f.pages }

// LittleEndian reports whether the file is stored little-endian.
func (f *File) LittleEndian() bool { return f.reader.order == binary.LittleEndian }

// BigTIFF reports whether the file uses BigTIFF (magic 43, 8-byte offsets).
func (f *File) BigTIFF() bool { return f.bigTIFF }

// ReaderAt returns the underlying reader for use by format packages reading
// tile byte ranges.
func (f *File) ReaderAt() io.ReaderAt { return f.r }

// Size returns the total byte size of the underlying reader as provided to Open.
func (f *File) Size() int64 { return f.size }
