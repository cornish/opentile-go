#!/usr/bin/env python3
"""Emit one tile or associated image from a slide via Python opentile to stdout.

Used by the Go parity harness (tests/oracle, built with -tags parity).
Never imported as a library.

Usage:
    oracle_runner.py <slide> <level> <x> <y>   # level tile
    oracle_runner.py <slide> <kind>            # associated image (label|overview|thumbnail)

Environment:
    OPENTILE_TILE_SIZE: requested output tile size (default 1024). Must
    match the WithTileSize option passed to Go's opentile.OpenFile.

Exit status:
    0 on success. For the associated-image form, if the requested kind is
    not present on the slide, exits 0 with zero-length stdout — the Go side
    treats that as "skip".
    2 on bad CLI usage.
"""
import os
import sys


def _tile_size() -> int:
    return int(os.environ.get("OPENTILE_TILE_SIZE", "1024"))


def _emit_level_tile(slide: str, level: int, x: int, y: int) -> int:
    from opentile import OpenTile

    # Python opentile 0.20.0 takes tile_size as int (see OpenTile.open
    # signature), not a tuple; passing a tuple trips NdpiTiler's tile-size
    # adjuster with a TypeError.
    tiler = OpenTile.open(slide, _tile_size())
    try:
        lvl = tiler.get_level(level)
        data = lvl.get_tile((x, y))
    finally:
        tiler.close()
    sys.stdout.buffer.write(data)
    return 0


def _emit_associated(slide: str, kind: str) -> int:
    from opentile import OpenTile

    if kind not in ("label", "overview", "thumbnail"):
        print(f"unknown associated kind: {kind}", file=sys.stderr)
        return 2

    tiler = OpenTile.open(slide, _tile_size())
    try:
        if kind == "label":
            imgs = tiler.labels
        elif kind == "overview":
            imgs = tiler.overviews
        else:  # thumbnail
            imgs = tiler.thumbnails

        if not imgs:
            # Kind not present on this slide — emit zero-length stdout, exit
            # status 0. Go side treats that as "skip".
            return 0

        # Python opentile exposes associated images via Image.get_tile((0, 0))
        # in 0.20.0 — there is no image_data property. get_tile((0, 0)) is the
        # full single-tile blob.
        data = imgs[0].get_tile((0, 0))
    finally:
        tiler.close()
    sys.stdout.buffer.write(data)
    return 0


def main() -> int:
    if len(sys.argv) == 5:
        slide, level_str, x_str, y_str = sys.argv[1:]
        try:
            level = int(level_str)
            x = int(x_str)
            y = int(y_str)
        except ValueError as e:
            print(f"bad arg: {e}", file=sys.stderr)
            return 2
        return _emit_level_tile(slide, level, x, y)
    if len(sys.argv) == 3:
        slide, kind = sys.argv[1:]
        return _emit_associated(slide, kind)
    print(
        "usage: oracle_runner.py <slide> <level> <x> <y>\n"
        "       oracle_runner.py <slide> <kind>",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
