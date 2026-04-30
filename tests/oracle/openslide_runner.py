#!/usr/bin/env python3
"""openslide-python parity oracle for Ventana BIF.

Started by the Go side once per slide. Drives openslide.OpenSlide
directly + decodes opentile-go's tile output via PIL (which uses the
same libjpeg-turbo backend as openslide). Cross-decoder pixel parity
without spurious noise from Go's stdlib image/jpeg vs libjpeg-turbo
IDCT differences.

Stdin protocol — request types:
    compare_tile <level> <col> <row> <tw> <th> <jpeg_byte_count>\\n
    <raw jpeg bytes>                            decode the uploaded JPEG
                                                 via PIL, compose the
                                                 corresponding openslide
                                                 read_region, and compare
                                                 pixel-by-pixel.
    quit                                         exit cleanly

Stdout protocol — one response per request:
    4-byte big-endian uint32 length, then that many bytes of blob.
    Zero-length = oracle could not run (tile out of grid, etc.).
    16-byte response = comparison result:
       byte 0     : 0 (match within threshold) | 1 (mismatch) | 2 (decode error)
       bytes 1..3 : reserved (zero)
       bytes 4..7 : uint32 BE — max per-channel absolute delta
       bytes 8..11: uint32 BE — count of pixel positions exceeding threshold
       bytes 12..15: uint32 BE — total pixel positions (= tw*th)

Stderr: human-readable diagnostics. Errors emit a length-zero stdout
response so the Go side can attribute the failure.
"""
import io
import struct
import sys

# Per-channel absolute-difference tolerance for the cross-decoder
# comparison. JPEG decoders from different libraries (Go's stdlib vs
# libjpeg-turbo) typically agree to within ±1; PIL and openslide-python
# share libjpeg-turbo, so deltas should be 0. We set a small tolerance
# anyway to absorb any future PIL upgrade quirks.
DELTA_THRESHOLD = 4


def _emit(out, data: bytes) -> None:
    out.write(struct.pack(">I", len(data)))
    if data:
        out.write(data)
    out.flush()


def _read_exact(stream, n: int) -> bytes:
    buf = bytearray()
    while len(buf) < n:
        chunk = stream.read(n - len(buf))
        if not chunk:
            raise EOFError(f"unexpected EOF reading {n} bytes (got {len(buf)})")
        buf.extend(chunk)
    return bytes(buf)


def _compare_tile(slide, Image, level, col, row, tw, th, jpeg_bytes):
    """Decode jpeg_bytes via PIL, compose openslide.read_region for the
    same (level, col, row, tw, th), and compare pixel-by-pixel.

    Returns (status, max_delta, mismatch_count, total_pixels) where
    status is 0 (match), 1 (mismatch), 2 (decode error), or 3 (tile
    extends past openslide's level extent — opentile-go uses the
    padded TIFF grid; openslide uses the AOI hull. The mismatch is
    by-design at the edge; skip).
    """
    if level < 0 or level >= slide.level_count:
        return 2, 0, 0, tw * th
    # openslide.level_dimensions[level] is (W, H) at that level.
    lw, lh = slide.level_dimensions[level]
    if (col + 1) * tw > lw or (row + 1) * th > lh:
        return 3, 0, 0, tw * th
    try:
        ours = Image.open(io.BytesIO(jpeg_bytes)).convert("RGBA")
    except Exception as e:
        print(f"PIL decode error: {e}", file=sys.stderr)
        return 2, 0, 0, tw * th
    if ours.size != (tw, th):
        print(
            f"size mismatch: PIL decoded {ours.size}, expected ({tw},{th})",
            file=sys.stderr,
        )
        return 1, 255, tw * th, tw * th
    scale = slide.level_downsamples[level]
    x = int(col * tw * scale)
    y = int(row * th * scale)
    theirs = slide.read_region((x, y), level, (tw, th))
    ours_b = ours.tobytes()
    theirs_b = theirs.tobytes()
    if len(ours_b) != len(theirs_b):
        return 1, 255, tw * th, tw * th
    max_delta = 0
    mismatch = 0
    # Pixel-by-pixel scan. Each pixel = 4 bytes (R,G,B,A). Walk channels;
    # track per-pixel max channel delta across the 4 channels.
    n = len(ours_b)
    o = ours_b
    t = theirs_b
    px_total = tw * th
    px_idx = 0
    while px_idx < px_total:
        base = px_idx * 4
        d0 = abs(o[base] - t[base])
        d1 = abs(o[base + 1] - t[base + 1])
        d2 = abs(o[base + 2] - t[base + 2])
        d3 = abs(o[base + 3] - t[base + 3])
        d = max(d0, d1, d2, d3)
        if d > max_delta:
            max_delta = d
        if d > DELTA_THRESHOLD:
            mismatch += 1
        px_idx += 1
    status = 0 if mismatch == 0 else 1
    return status, max_delta, mismatch, px_total


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: openslide_runner.py <slide>", file=sys.stderr)
        return 2
    slide_path = sys.argv[1]
    import openslide
    from PIL import Image

    slide = openslide.OpenSlide(slide_path)
    out = sys.stdout.buffer
    try:
        # Read line-based commands; binary uploads come immediately
        # after their header line.
        stdin_b = sys.stdin.buffer
        while True:
            line = stdin_b.readline()
            if not line:
                break
            line = line.strip().decode("ascii", errors="replace")
            if not line or line == "quit":
                break
            parts = line.split()
            try:
                if parts[0] == "compare_tile":
                    level = int(parts[1])
                    col = int(parts[2])
                    row = int(parts[3])
                    tw = int(parts[4])
                    th = int(parts[5])
                    jpeg_count = int(parts[6])
                    jpeg_bytes = _read_exact(stdin_b, jpeg_count)
                    status, mx, mc, total = _compare_tile(
                        slide, Image, level, col, row, tw, th, jpeg_bytes
                    )
                    blob = struct.pack(">BBBBIII", status, 0, 0, 0, mx, mc, total)
                else:
                    raise ValueError(f"unknown command: {parts[0]!r}")
            except Exception as e:
                print(f"openslide_runner: {line!r}: {e}", file=sys.stderr)
                blob = b""
            _emit(out, blob)
    finally:
        slide.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
