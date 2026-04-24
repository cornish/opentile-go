#!/usr/bin/env python3
"""Emit one tile from a slide using Python opentile to stdout.

Used by the Go parity harness (tests/oracle, built with -tags parity).
Never imported as a library.

Usage:
    oracle_runner.py <slide> <level> <x> <y>

Environment:
    OPENTILE_TILE_SIZE: requested output tile size (default 1024). Must
    match the WithTileSize option passed to Go's opentile.OpenFile.
"""
import os
import sys


def main() -> int:
    if len(sys.argv) != 5:
        print("usage: oracle_runner.py <slide> <level> <x> <y>", file=sys.stderr)
        return 2
    slide, level_str, x_str, y_str = sys.argv[1:]
    try:
        level = int(level_str)
        x = int(x_str)
        y = int(y_str)
    except ValueError as e:
        print(f"bad arg: {e}", file=sys.stderr)
        return 2
    from opentile import OpenTile

    tile_size = int(os.environ.get("OPENTILE_TILE_SIZE", "1024"))
    tiler = OpenTile.open(slide, (tile_size, tile_size))
    try:
        lvl = tiler.get_level(level)
        data = lvl.get_tile((x, y))
    finally:
        tiler.close()
    sys.stdout.buffer.write(data)
    return 0


if __name__ == "__main__":
    sys.exit(main())
