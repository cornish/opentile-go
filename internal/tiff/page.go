package tiff

// Page wraps a single parsed IFD with the byte reader needed to decode its tag
// values. Typed tag accessors are added in a subsequent task.
type Page struct {
	ifd *ifd
	br  *byteReader
}

func newPage(i *ifd, br *byteReader) *Page { return &Page{ifd: i, br: br} }
