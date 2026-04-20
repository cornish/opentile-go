package tiff

import (
	"io"
)

// File is a parsed TIFF file: a list of pages in IFD order, plus the reader
// and byte order needed to decode tag values and read tile payloads.
type File struct {
	r      io.ReaderAt
	reader *byteReader
	pages  []*Page
}

// Open parses the header and every IFD in r, producing a File ready for use by
// format packages. Open does not read tile payloads.
func Open(r io.ReaderAt) (*File, error) {
	h, err := parseHeader(r)
	if err != nil {
		return nil, err
	}
	br := newByteReader(r, h.littleEndian)
	ifds, err := walkIFDs(br, int64(h.firstIFD))
	if err != nil {
		return nil, err
	}
	pages := make([]*Page, 0, len(ifds))
	for _, i := range ifds {
		pages = append(pages, newPage(i, br))
	}
	return &File{r: r, reader: br, pages: pages}, nil
}

// Pages returns the pages in IFD order. The slice is owned by File; do not mutate.
func (f *File) Pages() []*Page { return f.pages }

// LittleEndian reports whether the file is stored little-endian.
func (f *File) LittleEndian() bool { return f.reader.order.String() == "LittleEndian" }

// ReaderAt returns the underlying reader for use by format packages reading
// tile byte ranges.
func (f *File) ReaderAt() io.ReaderAt { return f.r }
