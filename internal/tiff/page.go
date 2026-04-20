package tiff

type Page struct {
	ifd *ifd
	br  *byteReader
}

func newPage(i *ifd, br *byteReader) *Page { return &Page{ifd: i, br: br} }
