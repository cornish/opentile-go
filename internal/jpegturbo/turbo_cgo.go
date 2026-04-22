//go:build cgo && !nocgo

package jpegturbo

/*
#cgo pkg-config: libturbojpeg
#include <turbojpeg.h>
#include <stdlib.h>

// Small C helper keeps the Go code free of tjtransform struct literal.
static int go_tj_transform_crop(
    const unsigned char *src, unsigned long src_size,
    int x, int y, int w, int h,
    unsigned char **dst, unsigned long *dst_size
) {
    tjhandle h_ = tjInitTransform();
    if (h_ == NULL) {
        return -1;
    }
    tjtransform t;
    // Zero-init the struct; only the fields we touch matter.
    t.r.x = x;
    t.r.y = y;
    t.r.w = w;
    t.r.h = h;
    t.op = TJXOP_NONE;
    t.options = TJXOPT_CROP | TJXOPT_PERFECT;
    t.data = NULL;
    t.customFilter = NULL;

    int rc = tjTransform(h_, src, src_size, 1, dst, dst_size, &t, 0);
    tjDestroy(h_);
    return rc;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// Crop performs an MCU-aligned lossless crop of src using libjpeg-turbo's
// tjTransform with TJXOPT_CROP and TJXOPT_PERFECT. A region that is not
// MCU-aligned, or that extends past the source dimensions, is rejected by
// libjpeg-turbo with a non-zero return code and Crop returns an error.
func Crop(src []byte, r Region) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("jpegturbo: empty source")
	}
	var dst *C.uchar
	var dstSize C.ulong
	rc := C.go_tj_transform_crop(
		(*C.uchar)(unsafe.Pointer(&src[0])),
		C.ulong(len(src)),
		C.int(r.X), C.int(r.Y), C.int(r.Width), C.int(r.Height),
		&dst, &dstSize,
	)
	if rc != 0 {
		return nil, fmt.Errorf("jpegturbo: tjTransform returned rc=%d (non-MCU crop? malformed input?)", rc)
	}
	defer C.tjFree(dst)
	out := C.GoBytes(unsafe.Pointer(dst), C.int(dstSize))
	return out, nil
}
