package tiff

import (
	"fmt"
)

// Well-known TIFF tag IDs used by opentile-go.
const (
	TagImageWidth        uint16 = 256
	TagImageLength       uint16 = 257
	TagBitsPerSample     uint16 = 258
	TagCompression       uint16 = 259
	TagPhotometric       uint16 = 262
	TagImageDescription  uint16 = 270
	TagSamplesPerPixel   uint16 = 277
	TagXResolution       uint16 = 282
	TagYResolution       uint16 = 283
	TagResolutionUnit    uint16 = 296
	TagTileWidth         uint16 = 322
	TagTileLength        uint16 = 323
	TagTileOffsets       uint16 = 324
	TagTileByteCounts    uint16 = 325
	TagJPEGTables        uint16 = 347
	TagYCbCrSubSampling  uint16 = 530
	TagInterColorProfile uint16 = 34675
)

// Page wraps a parsed IFD and exposes typed accessors for the tags opentile-go
// needs. Missing tags return (zero, false) — callers decide whether the
// absence is fatal.
type Page struct {
	ifd *ifd
	br  *byteReader
}

func newPage(i *ifd, br *byteReader) *Page { return &Page{ifd: i, br: br} }

// scalarU32 returns the first value of a tag, or (0, false) if missing.
func (p *Page) scalarU32(tag uint16) (uint32, bool) {
	e, ok := p.ifd.get(tag)
	if !ok {
		return 0, false
	}
	vals, err := e.Values(p.br)
	if err != nil || len(vals) == 0 {
		return 0, false
	}
	return vals[0], true
}

// ScalarU32 returns the first value of an arbitrary tag as uint32, or
// (0, false) if the tag is absent. Exposed so format packages can read
// vendor-private tags without the internal helpers gaining per-tag
// accessors.
func (p *Page) ScalarU32(tag uint16) (uint32, bool) { return p.scalarU32(tag) }

func (p *Page) ImageWidth() (uint32, bool)       { return p.scalarU32(TagImageWidth) }
func (p *Page) ImageLength() (uint32, bool)      { return p.scalarU32(TagImageLength) }
func (p *Page) TileWidth() (uint32, bool)        { return p.scalarU32(TagTileWidth) }
func (p *Page) TileLength() (uint32, bool)       { return p.scalarU32(TagTileLength) }
func (p *Page) Compression() (uint32, bool)      { return p.scalarU32(TagCompression) }
func (p *Page) Photometric() (uint32, bool)      { return p.scalarU32(TagPhotometric) }
func (p *Page) SamplesPerPixel() (uint32, bool)  { return p.scalarU32(TagSamplesPerPixel) }
func (p *Page) BitsPerSample() (uint32, bool)    { return p.scalarU32(TagBitsPerSample) }
func (p *Page) ResolutionUnit() (uint32, bool)   { return p.scalarU32(TagResolutionUnit) }

// ImageDescription returns the ASCII ImageDescription tag if present.
func (p *Page) ImageDescription() (string, bool) {
	e, ok := p.ifd.get(TagImageDescription)
	if !ok {
		return "", false
	}
	s, err := e.decodeASCII(p.br, e.valueBytes[:])
	if err != nil {
		return "", false
	}
	return s, true
}

// JPEGTables returns the JPEG tables blob used as a prefix for tiles, if present.
func (p *Page) JPEGTables() ([]byte, bool) {
	e, ok := p.ifd.get(TagJPEGTables)
	if !ok {
		return nil, false
	}
	// Tables are UNDEFINED bytes; read the payload.
	if e.fitsInline() {
		return append([]byte(nil), e.valueBytes[:e.Count]...), true
	}
	buf, err := p.br.bytes(int64(e.valueOrOffset), int(e.Count))
	if err != nil {
		return nil, false
	}
	return buf, true
}

// ICCProfile returns the InterColorProfile tag bytes if present.
func (p *Page) ICCProfile() ([]byte, bool) {
	e, ok := p.ifd.get(TagInterColorProfile)
	if !ok {
		return nil, false
	}
	if e.fitsInline() {
		return append([]byte(nil), e.valueBytes[:e.Count]...), true
	}
	buf, err := p.br.bytes(int64(e.valueOrOffset), int(e.Count))
	if err != nil {
		return nil, false
	}
	return buf, true
}

// TileOffsets returns the TileOffsets array.
func (p *Page) TileOffsets() ([]uint32, error) {
	return p.arrayU32(TagTileOffsets)
}

// TileByteCounts returns the TileByteCounts array.
func (p *Page) TileByteCounts() ([]uint32, error) {
	return p.arrayU32(TagTileByteCounts)
}

func (p *Page) arrayU32(tag uint16) ([]uint32, error) {
	e, ok := p.ifd.get(tag)
	if !ok {
		return nil, fmt.Errorf("tiff: tag %d missing", tag)
	}
	return e.Values(p.br)
}

// TileOffsets64 returns the TileOffsets array as uint64 values; supports both
// LONG (classic TIFF) and LONG8 (BigTIFF) encodings.
func (p *Page) TileOffsets64() ([]uint64, error) {
	return p.arrayU64(TagTileOffsets)
}

// TileByteCounts64 returns the TileByteCounts array as uint64 values.
func (p *Page) TileByteCounts64() ([]uint64, error) {
	return p.arrayU64(TagTileByteCounts)
}

// ScalarArrayU64 returns the value array for an arbitrary tag as uint64s.
// Generalizes TileOffsets64/TileByteCounts64 for callers that need other
// array-valued tags (e.g., SVS StripOffsets, NDPI vendor arrays).
func (p *Page) ScalarArrayU64(tag uint16) ([]uint64, error) {
	return p.arrayU64(tag)
}

func (p *Page) arrayU64(tag uint16) ([]uint64, error) {
	e, ok := p.ifd.get(tag)
	if !ok {
		return nil, fmt.Errorf("tiff: tag %d missing", tag)
	}
	return e.Values64(p.br)
}

// XResolution returns the X resolution as a numerator/denominator rational.
func (p *Page) XResolution() (num, den uint32, ok bool) {
	return p.rationalFirst(TagXResolution)
}

// YResolution returns the Y resolution as a numerator/denominator rational.
func (p *Page) YResolution() (num, den uint32, ok bool) {
	return p.rationalFirst(TagYResolution)
}

func (p *Page) rationalFirst(tag uint16) (uint32, uint32, bool) {
	e, found := p.ifd.get(tag)
	if !found {
		return 0, 0, false
	}
	vals, err := e.decodeRational(p.br)
	if err != nil || len(vals) == 0 {
		return 0, 0, false
	}
	return vals[0][0], vals[0][1], true
}

// TileGrid returns the tile grid dimensions (tiles in X, tiles in Y).
// Result is computed via ceil division: a partial tile at the edge counts as one.
func (p *Page) TileGrid() (int, int, error) {
	iw, ok := p.ImageWidth()
	if !ok {
		return 0, 0, fmt.Errorf("tiff: ImageWidth missing")
	}
	il, ok := p.ImageLength()
	if !ok {
		return 0, 0, fmt.Errorf("tiff: ImageLength missing")
	}
	tw, ok := p.TileWidth()
	if !ok || tw == 0 {
		return 0, 0, fmt.Errorf("tiff: TileWidth missing or zero")
	}
	tl, ok := p.TileLength()
	if !ok || tl == 0 {
		return 0, 0, fmt.Errorf("tiff: TileLength missing or zero")
	}
	gx := int(iw / tw)
	if iw%tw != 0 {
		gx++
	}
	gy := int(il / tl)
	if il%tl != 0 {
		gy++
	}
	return gx, gy, nil
}
