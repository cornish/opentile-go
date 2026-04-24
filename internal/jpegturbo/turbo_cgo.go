//go:build cgo && !nocgo

package jpegturbo

/*
#cgo pkg-config: libturbojpeg
#include <turbojpeg.h>
#include <stdlib.h>
#include <string.h>

// go_tj_transform_crop runs tjTransform with TJXOPT_CROP|TJXOPT_PERFECT.
// On failure, copies tjGetErrorStr2's message into err_msg (up to err_cap-1
// bytes, NUL-terminated) before the handle is destroyed. Returns 0 on
// success, non-zero on any failure.
//
// Caller owns: freeing *dst with tjFree iff *dst != NULL, regardless of rc.
static int go_tj_transform_crop(
    const unsigned char *src, unsigned long src_size,
    int x, int y, int w, int h,
    unsigned char **dst, unsigned long *dst_size,
    char *err_msg, int err_cap
) {
    if (err_msg != NULL && err_cap > 0) {
        err_msg[0] = '\0';
    }
    tjhandle h_ = tjInitTransform();
    if (h_ == NULL) {
        if (err_msg != NULL && err_cap > 0) {
            // tjGetErrorStr2 requires a handle; use the legacy tjGetErrorStr
            // when handle init failed.
            const char *m = tjGetErrorStr();
            if (m != NULL) {
                strncpy(err_msg, m, err_cap - 1);
                err_msg[err_cap - 1] = '\0';
            }
        }
        return -1;
    }
    tjtransform t = {0};
    t.r.x = x;
    t.r.y = y;
    t.r.w = w;
    t.r.h = h;
    t.op = TJXOP_NONE;
    t.options = TJXOPT_CROP | TJXOPT_PERFECT;

    int rc = tjTransform(h_, src, src_size, 1, dst, dst_size, &t, 0);
    if (rc != 0 && err_msg != NULL && err_cap > 0) {
        const char *m = tjGetErrorStr2(h_);
        if (m != NULL) {
            strncpy(err_msg, m, err_cap - 1);
            err_msg[err_cap - 1] = '\0';
        }
    }
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
//
// Concurrency: Crop creates and destroys a fresh tjhandle per call, so
// concurrent invocations from different goroutines do not share mutable
// state. libjpeg-turbo's tjInitTransform/tjDestroy are reentrant.
//
// See CropWithBackground for a variant that tolerates out-of-bounds crops
// by filling the OOB blocks with a background color.
func Crop(src []byte, r Region) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("jpegturbo: empty source")
	}
	var dst *C.uchar
	var dstSize C.ulong

	// Error message buffer. tjGetErrorStr2 messages are typically short;
	// 256 bytes is ample.
	const errBufLen = 256
	errBuf := make([]byte, errBufLen)
	errPtr := (*C.char)(unsafe.Pointer(&errBuf[0]))

	rc := C.go_tj_transform_crop(
		(*C.uchar)(unsafe.Pointer(&src[0])),
		C.ulong(len(src)),
		C.int(r.X), C.int(r.Y), C.int(r.Width), C.int(r.Height),
		&dst, &dstSize,
		errPtr, C.int(errBufLen),
	)
	// Always free dst when non-NULL — libjpeg-turbo may have allocated it
	// partially before an error is detected.
	if dst != nil {
		defer C.tjFree(dst)
	}
	if rc != 0 {
		msg := C.GoString(errPtr)
		if msg == "" {
			msg = "(no message)"
		}
		return nil, fmt.Errorf("jpegturbo: tjTransform failed (rc=%d): %s", rc, msg)
	}
	out := C.GoBytes(unsafe.Pointer(dst), C.int(dstSize))
	return out, nil
}

// CropWithBackground behaves like Crop but tolerates crop regions that
// extend past the source image. In v0.2 this is a stub that delegates to
// Crop when the region fits inside the image and returns an error otherwise;
// the real DCT-callback-based implementation lands in a follow-up commit.
func CropWithBackground(src []byte, r Region) ([]byte, error) {
	// Scaffold: v0.2 intentionally falls through to Crop. The real body
	// (fill_background CUSTOMFILTER callback) ships in the next commit.
	return Crop(src, r)
}
