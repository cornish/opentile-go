#!/usr/bin/env python3
"""tifffile-based parity oracle for OME-TIFF.

Started by the Go side once per slide. Drives tifffile directly to read raw
tile bytes from any TIFF page — useful for multi-image OME files where
opentile-py drops 3 of 4 main pyramids via its last-wins loop, and we need
a different reference to byte-validate our exposed pyramids.

Stdin protocol — one line per request:
    tile <imageIdx> <levelIdx> <x> <y>     emit raw tile bytes for a
                                            tiled-level position
    quit                                    exit cleanly

Stdout protocol — one response per request:
    4-byte big-endian uint32 length, then that many bytes of blob.
    Zero-length response = "skip / not applicable".

imageIdx is the 0-based document-order index of a main-pyramid series
(macro / label / thumbnail series are excluded, matching opentile-go's
exposure). levelIdx is the 0-based level inside that series's pyramid;
0 = baseline. Only TILED levels are supported (OneFrame levels have no
straight-byte reference; their parity is via integration fixtures).

Stderr: human-readable diagnostics. Errors during a request emit a
length-zero stdout response so the Go side can attribute the failure.
"""
import struct
import sys


class _NotTiledLevel(Exception):
    """Sentinel raised when a level is non-tiled (OneFrame). Caller emits a
    zero-length response without printing to stderr — the Go side already
    expects this sentinel via the zero-length path."""


def _emit(out, data: bytes) -> None:
    out.write(struct.pack(">I", len(data)))
    if data:
        out.write(data)
    out.flush()


def _main_series_indices(tf) -> list:
    """Series indices for main pyramids (everything except associated)."""
    out = []
    for i, s in enumerate(tf.series):
        if s.name.strip() in ("macro", "label", "thumbnail"):
            continue
        out.append(i)
    return out


def _tile_raw_bytes(tf, file_handle, image_idx: int, level_idx: int,
                    x: int, y: int) -> bytes:
    """Read raw tile bytes for the given (image, level, x, y) position via
    tifffile's offsets/counts arrays. Plane 0 only when
    PlanarConfiguration=2; matches the indexing opentile-go uses."""
    main = _main_series_indices(tf)
    if image_idx < 0 or image_idx >= len(main):
        raise IndexError(f"image index {image_idx} out of range (have {len(main)})")
    series = tf.series[main[image_idx]]
    if level_idx < 0 or level_idx >= len(series.levels):
        raise IndexError(f"level index {level_idx} out of range (have {len(series.levels)})")
    page = series.levels[level_idx].pages[0]
    if not page.tilewidth:
        # Expected on OneFrame levels; no straight-byte tifffile reference
        # available. Surface as a sentinel exception the runner suppresses
        # silently so test output stays quiet.
        raise _NotTiledLevel()
    grid_w = (page.imagewidth + page.tilewidth - 1) // page.tilewidth
    grid_h = (page.imagelength + page.tilelength - 1) // page.tilelength
    if x < 0 or y < 0 or x >= grid_w or y >= grid_h:
        raise IndexError(f"tile ({x},{y}) out of grid {grid_w}x{grid_h}")
    idx = y * grid_w + x  # plane 0 indexing
    if idx >= len(page.dataoffsets):
        raise IndexError(f"flat index {idx} >= dataoffsets length {len(page.dataoffsets)}")
    file_handle.seek(page.dataoffsets[idx])
    return file_handle.read(page.databytecounts[idx])


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: tifffile_runner.py <slide>", file=sys.stderr)
        return 2
    slide = sys.argv[1]
    import tifffile

    tf = tifffile.TiffFile(slide)
    file_handle = open(slide, "rb")
    out = sys.stdout.buffer
    try:
        for line in sys.stdin:
            line = line.strip()
            if not line or line == "quit":
                break
            parts = line.split()
            try:
                if parts[0] == "tile":
                    ii = int(parts[1])
                    li = int(parts[2])
                    x = int(parts[3])
                    y = int(parts[4])
                    data = _tile_raw_bytes(tf, file_handle, ii, li, x, y)
                else:
                    raise ValueError(f"unknown command: {parts[0]!r}")
            except _NotTiledLevel:
                data = b""
            except Exception as e:
                print(f"tifffile_runner: {line!r}: {e}", file=sys.stderr)
                data = b""
            _emit(out, data)
    finally:
        file_handle.close()
        tf.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
