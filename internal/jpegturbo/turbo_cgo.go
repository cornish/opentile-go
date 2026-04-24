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

// Data passed to go_tj_fill_background via tjtransform.data. Mirrors
// PyTurboJPEG's BackgroundStruct: image dimensions in pixels plus the
// quantised DC coefficient to write for out-of-bounds blocks.
typedef struct fill_bg_data {
    int w;
    int h;
    int lum; // DC coefficient (post-quantisation) to plant in OOB luma blocks
} fill_bg_data;

// go_tj_fill_background is a CUSTOMFILTER callback that zeros the DCT
// coefficients of blocks whose pixel position is past (w, h) and sets
// their DC coefficient to `lum`. This lets tjTransform tolerate crop
// regions that extend past the source image by filling the OOB area
// with a solid color in the DCT domain.
//
// Port of PyTurboJPEG's fill_background (turbojpeg.py:202-287). Component
// 0 (luma) is filled; chroma components are left alone — for luma-only
// fills the chroma DC will bias the color slightly toward neutral, which
// matches Python opentile's default behaviour.
//
// MCU sizes in the coeff array are always 8x8 regardless of JPEG
// subsampling (libjpeg-turbo normalizes per-component blocks).
#define MCU_SIDE 8
#define MCU_AREA 64

static int go_tj_fill_background(
    short *coeffs,
    tjregion arrayRegion,
    tjregion planeRegion,
    int componentID,
    int transformID,
    struct tjtransform *transform
) {
    (void)transformID;
    if (componentID != 0) return 1; // only touch luma
    fill_bg_data *d = (fill_bg_data *)transform->data;
    if (d == NULL) return 1;

    int aw = arrayRegion.w;
    int ah = arrayRegion.h;
    int ax = arrayRegion.x;
    int ay = arrayRegion.y;
    int pw = planeRegion.w;
    // planeRegion.h is not needed below; the "under" pass uses ay+ah.

    // coeffs is laid out as (ah/8) rows of (aw/8) blocks, 64 shorts each.
    // Access: block(row, col)[0..63], DC = [0], AC = [1..63].
    int blocks_per_row = aw / MCU_SIDE;

    // Fill MCUs to the RIGHT of the original image (columns ≥ d->w/8 in
    // plane coords) for each row within arrayRegion that is still inside
    // the original image vertically (rows < d->h/8 in plane coords).
    int left_start_row_px = ay; if (left_start_row_px > d->h) left_start_row_px = d->h;
    left_start_row_px -= ay;
    int left_end_row_px = ay + ah; if (left_end_row_px > d->h) left_end_row_px = d->h;
    left_end_row_px -= ay;

    for (int bx = d->w / MCU_SIDE; bx < pw / MCU_SIDE; bx++) {
        // Convert plane-coord block X to array-coord block X.
        int arr_bx = bx - (ax / MCU_SIDE);
        if (arr_bx < 0 || arr_bx >= blocks_per_row) continue;
        for (int by = left_start_row_px / MCU_SIDE; by < left_end_row_px / MCU_SIDE; by++) {
            short *blk = coeffs + (by * blocks_per_row + arr_bx) * MCU_AREA;
            // Zero all coefficients; plant DC.
            for (int k = 0; k < MCU_AREA; k++) blk[k] = 0;
            blk[0] = (short)d->lum;
        }
    }

    // Fill MCUs BELOW the original image (rows ≥ d->h/8 in plane coords)
    // across the entire array width.
    int bottom_start_row_px = ay; if (bottom_start_row_px < d->h) bottom_start_row_px = d->h;
    bottom_start_row_px -= ay;
    int bottom_end_row_px = ay + ah; if (bottom_end_row_px < d->h) bottom_end_row_px = d->h;
    bottom_end_row_px -= ay;

    for (int bx = 0; bx < pw / MCU_SIDE; bx++) {
        int arr_bx = bx - (ax / MCU_SIDE);
        if (arr_bx < 0 || arr_bx >= blocks_per_row) continue;
        for (int by = bottom_start_row_px / MCU_SIDE; by < bottom_end_row_px / MCU_SIDE; by++) {
            short *blk = coeffs + (by * blocks_per_row + arr_bx) * MCU_AREA;
            for (int k = 0; k < MCU_AREA; k++) blk[k] = 0;
            blk[0] = (short)d->lum;
        }
    }

    return 1;
}

// go_tj_transform_crop_fill is like go_tj_transform_crop but attaches a
// fill-background CUSTOMFILTER callback. img_w and img_h are the
// ORIGINAL image dimensions (what the SOF advertises on input);
// libjpeg-turbo needs these so the callback can decide which blocks are
// OOB.
//
// `lum` is the post-quantisation DC coefficient written to OOB luma
// blocks. Pass 0 for a black fill. A positive value approximates a
// brighter fill; the exact white-equivalent requires reading DQT[0] and
// computing `round((luminance * 2047 - 1024) / dc_quant)` per
// PyTurboJPEG's __map_luminance_to_dc_dct_coefficient — see L12 in
// docs/deferred.md.
static int go_tj_transform_crop_fill(
    const unsigned char *src, unsigned long src_size,
    int x, int y, int w, int h,
    int img_w, int img_h, int lum,
    unsigned char **dst, unsigned long *dst_size,
    char *err_msg, int err_cap
) {
    if (err_msg != NULL && err_cap > 0) {
        err_msg[0] = '\0';
    }
    tjhandle h_ = tjInitTransform();
    if (h_ == NULL) {
        if (err_msg != NULL && err_cap > 0) {
            const char *m = tjGetErrorStr();
            if (m != NULL) {
                strncpy(err_msg, m, err_cap - 1);
                err_msg[err_cap - 1] = '\0';
            }
        }
        return -1;
    }

    fill_bg_data fd;
    fd.w = img_w;
    fd.h = img_h;
    fd.lum = lum;

    tjtransform t = {0};
    t.r.x = x;
    t.r.y = y;
    t.r.w = w;
    t.r.h = h;
    t.op = TJXOP_NONE;
    t.options = TJXOPT_CROP | TJXOPT_PERFECT;
    t.data = &fd;
    t.customFilter = go_tj_fill_background;

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

// go_tj_header_dims reads the SOF of src via tjDecompressHeader3 and
// returns the image width/height. Returns 0 on success, non-zero on
// failure; error message copied into err_msg when non-NULL.
static int go_tj_header_dims(
    const unsigned char *src, unsigned long src_size,
    int *out_w, int *out_h,
    char *err_msg, int err_cap
) {
    if (err_msg != NULL && err_cap > 0) {
        err_msg[0] = '\0';
    }
    tjhandle h_ = tjInitDecompress();
    if (h_ == NULL) {
        if (err_msg != NULL && err_cap > 0) {
            const char *m = tjGetErrorStr();
            if (m != NULL) {
                strncpy(err_msg, m, err_cap - 1);
                err_msg[err_cap - 1] = '\0';
            }
        }
        return -1;
    }
    int subsamp = 0;
    int colorspace = 0;
    int rc = tjDecompressHeader3(h_, src, src_size, out_w, out_h, &subsamp, &colorspace);
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
// extend past the source image. Out-of-bounds DCT blocks are filled via
// a CUSTOMFILTER callback. Required for NDPI edge tiles, where the level
// JPEG's dimensions are not an integer multiple of the output tile size.
//
// Fill behavior: luma DCT blocks outside the original (width, height)
// rectangle are zeroed, leaving a neutral-gray fill. PyTurboJPEG's
// default is `background_luminance=1.0` (white) which requires
// quantisation-table math; porting that is tracked as L12 in
// docs/deferred.md. Black/zero fill is adequate for v0.2 since the
// visible area of the cropped tile is bit-identical with Python; only
// OOB pixels differ.
//
// Concurrency: as Crop — fresh tjhandle per call; safe for parallel use.
func CropWithBackground(src []byte, r Region) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("jpegturbo: empty source")
	}
	// Read the source image dimensions so the callback knows what's OOB.
	var imgW, imgH C.int
	const errBufLen = 256
	errBuf := make([]byte, errBufLen)
	errPtr := (*C.char)(unsafe.Pointer(&errBuf[0]))

	rcDim := C.go_tj_header_dims(
		(*C.uchar)(unsafe.Pointer(&src[0])),
		C.ulong(len(src)),
		&imgW, &imgH,
		errPtr, C.int(errBufLen),
	)
	if rcDim != 0 {
		msg := C.GoString(errPtr)
		if msg == "" {
			msg = "(no message)"
		}
		return nil, fmt.Errorf("jpegturbo: tjDecompressHeader3 failed (rc=%d): %s", rcDim, msg)
	}

	var dst *C.uchar
	var dstSize C.ulong

	// Re-zero the error buffer.
	for i := range errBuf {
		errBuf[i] = 0
	}

	// lum=0 → black fill. See godoc above.
	rc := C.go_tj_transform_crop_fill(
		(*C.uchar)(unsafe.Pointer(&src[0])),
		C.ulong(len(src)),
		C.int(r.X), C.int(r.Y), C.int(r.Width), C.int(r.Height),
		imgW, imgH, C.int(0),
		&dst, &dstSize,
		errPtr, C.int(errBufLen),
	)
	if dst != nil {
		defer C.tjFree(dst)
	}
	if rc != 0 {
		msg := C.GoString(errPtr)
		if msg == "" {
			msg = "(no message)"
		}
		return nil, fmt.Errorf("jpegturbo: tjTransform (fill) failed (rc=%d): %s", rc, msg)
	}
	out := C.GoBytes(unsafe.Pointer(dst), C.int(dstSize))
	return out, nil
}
