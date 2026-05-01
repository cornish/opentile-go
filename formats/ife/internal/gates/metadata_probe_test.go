//go:build gates

package gates

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestT8Metadata dumps the cervix METADATA block + every sub-block
// (ATTRIBUTES, IMAGE_ARRAY, ICC_PROFILE, ANNOTATIONS) so we can scope
// the metadata-extraction work realistically.
//
// Per upstream Iris-File-Extension/IrisCodecExtension.hpp:
//
//	METADATA header (56 B):
//	  off  0  u64 validation     == self offset
//	  off  8  u16 recovery       (RECOVER_METADATA = 0x5504)
//	  off 10  u16 codec_major
//	  off 12  u16 codec_minor
//	  off 14  u16 codec_build
//	  off 16  u64 attributes_offset
//	  off 24  u64 images_offset
//	  off 32  u64 icc_color_offset
//	  off 40  u64 annotations_offset
//	  off 48  f32 microns_per_pixel
//	  off 52  f32 magnification
func TestT8Metadata(t *testing.T) {
	dir := os.Getenv("OPENTILE_TESTDIR")
	if dir == "" {
		t.Skip("OPENTILE_TESTDIR not set")
	}
	path := filepath.Join(dir, "ife", "cervix_2x_jpeg.iris")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	stat, _ := f.Stat()
	fileSize := stat.Size()

	// FILE_HEADER → metadata_offset.
	hdr := make([]byte, 38)
	if _, err := f.ReadAt(hdr, 0); err != nil {
		t.Fatalf("ReadAt FILE_HEADER: %v", err)
	}
	mdOff := binary.LittleEndian.Uint64(hdr[30:38])
	t.Logf("METADATA block @ 0x%x", mdOff)

	mdBuf := make([]byte, 56)
	if _, err := f.ReadAt(mdBuf, int64(mdOff)); err != nil {
		t.Fatalf("ReadAt METADATA: %v", err)
	}
	t.Logf("  raw 56 B: %s", hex.EncodeToString(mdBuf))

	validation := binary.LittleEndian.Uint64(mdBuf[0:8])
	recovery := binary.LittleEndian.Uint16(mdBuf[8:10])
	codecMajor := binary.LittleEndian.Uint16(mdBuf[10:12])
	codecMinor := binary.LittleEndian.Uint16(mdBuf[12:14])
	codecBuild := binary.LittleEndian.Uint16(mdBuf[14:16])
	attrsOff := binary.LittleEndian.Uint64(mdBuf[16:24])
	imagesOff := binary.LittleEndian.Uint64(mdBuf[24:32])
	iccOff := binary.LittleEndian.Uint64(mdBuf[32:40])
	annOff := binary.LittleEndian.Uint64(mdBuf[40:48])
	mpp := math.Float32frombits(binary.LittleEndian.Uint32(mdBuf[48:52]))
	mag := math.Float32frombits(binary.LittleEndian.Uint32(mdBuf[52:56]))

	t.Logf("  validation       = 0x%016x (want 0x%016x)", validation, mdOff)
	t.Logf("  recovery         = 0x%04x (want 0x5504 RECOVER_METADATA)", recovery)
	t.Logf("  codec_version    = %d.%d.%d", codecMajor, codecMinor, codecBuild)
	t.Logf("  attributes_off   = %s", offStr(attrsOff))
	t.Logf("  images_off       = %s", offStr(imagesOff))
	t.Logf("  icc_color_off    = %s", offStr(iccOff))
	t.Logf("  annotations_off  = %s", offStr(annOff))
	t.Logf("  microns_per_px   = %g", mpp)
	t.Logf("  magnification    = %g", mag)

	const NULL = uint64(0xFFFFFFFFFFFFFFFF)

	if attrsOff != NULL && attrsOff < uint64(fileSize) {
		dumpAttributes(t, f, attrsOff)
	}
	if imagesOff != NULL && imagesOff < uint64(fileSize) {
		dumpImages(t, f, imagesOff)
	}
	if iccOff != NULL && iccOff < uint64(fileSize) {
		dumpICC(t, f, iccOff)
	}
	if annOff != NULL && annOff < uint64(fileSize) {
		dumpAnnotations(t, f, annOff)
	}
}

func offStr(o uint64) string {
	if o == 0xFFFFFFFFFFFFFFFF {
		return "NULL"
	}
	return fmt.Sprintf("0x%016x", o)
}

// ATTRIBUTES (35 B header):
//
//	off  0  u64 validation
//	off  8  u16 recovery
//	off 10  u8  format  (0=undef, 1=I2S, 2=DICOM)
//	off 11  u16 version
//	off 13  u64 lengths_offset (-> ATTRIBUTES_SIZES)
//	off 21  u64 byte_array_offset (-> ATTRIBUTES_BYTES)
//
// ATTRIBUTES_SIZES (14 B header + N×6 B):
//
//	header: validation:u64, recovery:u16, entry_size:u16, entry_n:u32
//	entry:  key_size:u16, value_size:u32
//
// ATTRIBUTES_BYTES (14 B header + concatenated bytes):
//
//	header: validation:u64, recovery:u16, entry_n:u32
//	body:   key0_bytes ... value0_bytes key1_bytes ... value1_bytes ...
func dumpAttributes(t *testing.T, f *os.File, off uint64) {
	t.Logf("ATTRIBUTES @ 0x%x:", off)
	hdr := make([]byte, 29)
	if _, err := f.ReadAt(hdr, int64(off)); err != nil {
		t.Logf("  ReadAt: %v", err)
		return
	}
	val := binary.LittleEndian.Uint64(hdr[0:8])
	rec := binary.LittleEndian.Uint16(hdr[8:10])
	fmtType := hdr[10]
	ver := binary.LittleEndian.Uint16(hdr[11:13])
	lenOff := binary.LittleEndian.Uint64(hdr[13:21])
	bytesOff := binary.LittleEndian.Uint64(hdr[21:29])
	t.Logf("  validation=0x%016x (want 0x%016x)", val, off)
	t.Logf("  recovery=0x%04x format=%d version=%d", rec, fmtType, ver)
	t.Logf("  lengths_offset=0x%016x byte_array_offset=0x%016x", lenOff, bytesOff)

	// ATTRIBUTES_SIZES.
	szHdr := make([]byte, 16)
	if _, err := f.ReadAt(szHdr, int64(lenOff)); err != nil {
		return
	}
	entrySize := binary.LittleEndian.Uint16(szHdr[10:12])
	entryN := binary.LittleEndian.Uint32(szHdr[12:16])
	t.Logf("  ATTRIBUTES_SIZES @ 0x%x: entry_size=%d entry_n=%d", lenOff, entrySize, entryN)

	if entryN == 0 || entrySize != 6 {
		return
	}
	szBuf := make([]byte, int(entryN)*int(entrySize))
	if _, err := f.ReadAt(szBuf, int64(lenOff)+16); err != nil {
		return
	}

	// ATTRIBUTES_BYTES (header is 14 bytes per upstream).
	bytesHdr := make([]byte, 14)
	if _, err := f.ReadAt(bytesHdr, int64(bytesOff)); err != nil {
		return
	}
	t.Logf("  ATTRIBUTES_BYTES @ 0x%x", bytesOff)

	// Read each (key, value) pair.
	bodyStart := int64(bytesOff) + 14
	cursor := bodyStart
	for i := uint32(0); i < entryN; i++ {
		ks := binary.LittleEndian.Uint16(szBuf[i*6 : i*6+2])
		vs := binary.LittleEndian.Uint32(szBuf[i*6+2 : i*6+6])
		key := make([]byte, ks)
		val := make([]byte, vs)
		if _, err := f.ReadAt(key, cursor); err != nil {
			return
		}
		cursor += int64(ks)
		if _, err := f.ReadAt(val, cursor); err != nil {
			return
		}
		cursor += int64(vs)
		// Show first 80 chars of each side to keep log readable.
		ksh := string(key)
		if len(ksh) > 80 {
			ksh = ksh[:77] + "..."
		}
		vsh := string(val)
		if len(vsh) > 120 {
			vsh = vsh[:117] + "..."
		}
		t.Logf("  attr[%d]: key=%q (len=%d) value=%q (len=%d)", i, ksh, ks, vsh, vs)
	}
}

// IMAGE_ARRAY (22 B header + N×24 B IMAGE_ENTRY):
//
//	header: validation:u64, recovery:u16, entry_size:u16, entry_n:u32  -- but also has codec_version / orientation per upstream? Try both.
//
// IMAGE_ENTRY (24 B): bytes_offset:u64, width:u32, height:u32,
//
//	encoding:u8, format:u8, orientation:u16
func dumpImages(t *testing.T, f *os.File, off uint64) {
	t.Logf("IMAGE_ARRAY @ 0x%x:", off)
	hdr := make([]byte, 22)
	if _, err := f.ReadAt(hdr, int64(off)); err != nil {
		t.Logf("  ReadAt: %v", err)
		return
	}
	t.Logf("  raw 22 B: %s", hex.EncodeToString(hdr))
	val := binary.LittleEndian.Uint64(hdr[0:8])
	rec := binary.LittleEndian.Uint16(hdr[8:10])
	es := binary.LittleEndian.Uint16(hdr[10:12])
	en := binary.LittleEndian.Uint32(hdr[12:16])
	t.Logf("  validation=0x%016x (want 0x%016x)", val, off)
	t.Logf("  recovery=0x%04x entry_size=%d entry_n=%d", rec, es, en)

	if en == 0 {
		return
	}
	body := make([]byte, int(en)*int(es))
	if _, err := f.ReadAt(body, int64(off)+16); err != nil {
		return
	}
	for i := uint32(0); i < en; i++ {
		base := i * uint32(es)
		bytesOff := binary.LittleEndian.Uint64(body[base : base+8])
		w := binary.LittleEndian.Uint32(body[base+8 : base+12])
		h := binary.LittleEndian.Uint32(body[base+12 : base+16])
		enc := body[base+16]
		fmtCode := body[base+17]
		orient := binary.LittleEndian.Uint16(body[base+18 : base+20])
		t.Logf("  image[%d]: bytes_off=0x%x w=%d h=%d encoding=%d format=%d orientation=0x%04x",
			i, bytesOff, w, h, enc, fmtCode, orient)

		// IMAGE_BYTES header: validation:u64 + recovery:u16 + title_size:u16 + image_size:u32 = 16 B.
		ibHdr := make([]byte, 16)
		if _, err := f.ReadAt(ibHdr, int64(bytesOff)); err != nil {
			continue
		}
		ibVal := binary.LittleEndian.Uint64(ibHdr[0:8])
		ibRec := binary.LittleEndian.Uint16(ibHdr[8:10])
		titleSize := binary.LittleEndian.Uint16(ibHdr[10:12])
		imageSize := binary.LittleEndian.Uint32(ibHdr[12:16])
		t.Logf("    IMAGE_BYTES @ 0x%x: validation=0x%016x recovery=0x%04x title_size=%d image_size=%d",
			bytesOff, ibVal, ibRec, titleSize, imageSize)
		if titleSize > 0 && titleSize < 256 {
			title := make([]byte, titleSize)
			f.ReadAt(title, int64(bytesOff)+16)
			t.Logf("    title=%q", string(title))
		}
		// Sniff the first 8 image bytes to confirm encoding marker.
		first := make([]byte, 8)
		if _, err := f.ReadAt(first, int64(bytesOff)+16+int64(titleSize)); err == nil {
			t.Logf("    image bytes start: %s (JPEG SOI=ff d8; PNG=89 50 4e 47)", hex.EncodeToString(first))
		}
	}
}

// ICC_PROFILE (14 B header + raw bytes):
//
//	validation:u64, recovery:u16, entry_n:u32
func dumpICC(t *testing.T, f *os.File, off uint64) {
	t.Logf("ICC_PROFILE @ 0x%x:", off)
	hdr := make([]byte, 14)
	if _, err := f.ReadAt(hdr, int64(off)); err != nil {
		return
	}
	val := binary.LittleEndian.Uint64(hdr[0:8])
	rec := binary.LittleEndian.Uint16(hdr[8:10])
	en := binary.LittleEndian.Uint32(hdr[10:14])
	t.Logf("  validation=0x%016x (want 0x%016x) recovery=0x%04x entry_n=%d", val, off, rec, en)
}

func dumpAnnotations(t *testing.T, f *os.File, off uint64) {
	t.Logf("ANNOTATIONS @ 0x%x: (skipping detailed dump; out of v0.8 scope)", off)
	hdr := make([]byte, 16)
	if _, err := f.ReadAt(hdr, int64(off)); err != nil {
		return
	}
	en := binary.LittleEndian.Uint32(hdr[12:16])
	t.Logf("  entry_n = %d", en)
}
