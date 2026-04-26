#!/usr/bin/env python3
"""Persistent batched runner for the Go parity oracle.

Started by the Go side once per slide; reads requests from stdin and emits
length-prefixed blobs on stdout. Drops the ~200ms Python + opentile import
cost from per-tile to per-slide.

Usage:
    oracle_runner.py <slide>

Stdin protocol — one line per request, terminated with "\\n":
    level <int> <x> <y>     emit level tile bytes
    associated <kind>       emit associated image of given kind ("label",
                            "overview", or "thumbnail"); zero-length response
                            when the slide doesn't expose that kind
    quit                    exit cleanly

Stdout protocol — one response per request:
    4-byte big-endian uint32 length, then that many bytes of blob.
    A zero-length response means "not exposed / skip".

Stderr: human-readable diagnostics. Errors during a request emit a length-zero
stdout response so the Go side can decide whether to fail or skip. Fatal
problems (subprocess can't import opentile, slide can't be opened) cause a
non-zero exit.

Backward compatibility: the v0.2 one-shot CLI form is preserved when argv has
the old shape — `oracle_runner.py <slide> <level> <x> <y>` or
`oracle_runner.py <slide> <kind>` — so the existing oracle.Tile / .Associated
helpers in oracle.go keep working without migration.

Environment:
    OPENTILE_TILE_SIZE: requested output tile size (default 1024). Must match
    the WithTileSize option passed to Go's opentile.OpenFile.
"""
import os
import struct
import sys


def _tile_size() -> int:
    return int(os.environ.get("OPENTILE_TILE_SIZE", "1024"))


def _emit(out, data: bytes) -> None:
    out.write(struct.pack(">I", len(data)))
    if data:
        out.write(data)
    out.flush()


def _associated_for(tiler, kind: str):
    if kind == "label":
        return tiler.labels
    if kind == "overview":
        return tiler.overviews
    if kind == "thumbnail":
        return tiler.thumbnails
    return []


def _run_batched(slide: str) -> int:
    from opentile import OpenTile

    tiler = OpenTile.open(slide, _tile_size())
    out = sys.stdout.buffer
    try:
        for line in sys.stdin:
            line = line.strip()
            if not line or line == "quit":
                break
            parts = line.split()
            try:
                if parts[0] == "level":
                    level = int(parts[1])
                    x = int(parts[2])
                    y = int(parts[3])
                    data = tiler.get_level(level).get_tile((x, y))
                elif parts[0] == "associated":
                    kind = parts[1]
                    imgs = _associated_for(tiler, kind)
                    data = imgs[0].get_tile((0, 0)) if imgs else b""
                else:
                    raise ValueError(f"unknown command: {parts[0]!r}")
            except Exception as e:
                # Length-zero response signals "skip / error"; details on stderr
                # so the Go side can attribute the failure if it cares.
                print(f"oracle_runner: {line!r}: {e}", file=sys.stderr)
                data = b""
            _emit(out, data)
    finally:
        tiler.close()
    return 0


def _legacy_one_shot_tile(slide: str, level: int, x: int, y: int) -> int:
    from opentile import OpenTile

    tiler = OpenTile.open(slide, _tile_size())
    try:
        lvl = tiler.get_level(level)
        data = lvl.get_tile((x, y))
    finally:
        tiler.close()
    sys.stdout.buffer.write(data)
    return 0


def _legacy_one_shot_associated(slide: str, kind: str) -> int:
    if kind not in ("label", "overview", "thumbnail"):
        print(f"unknown associated kind: {kind}", file=sys.stderr)
        return 2
    from opentile import OpenTile

    tiler = OpenTile.open(slide, _tile_size())
    try:
        imgs = _associated_for(tiler, kind)
        if not imgs:
            return 0  # zero-length stdout = "skip"
        data = imgs[0].get_tile((0, 0))
    finally:
        tiler.close()
    sys.stdout.buffer.write(data)
    return 0


def main() -> int:
    # Batched form: exactly one positional argument (the slide path).
    if len(sys.argv) == 2:
        return _run_batched(sys.argv[1])
    # Legacy v0.2 one-shot forms (kept for back-compat with oracle.Tile /
    # oracle.Associated until callers migrate to oracle.NewSession).
    if len(sys.argv) == 5:
        slide, level_str, x_str, y_str = sys.argv[1:]
        try:
            level = int(level_str)
            x = int(x_str)
            y = int(y_str)
        except ValueError as e:
            print(f"bad arg: {e}", file=sys.stderr)
            return 2
        return _legacy_one_shot_tile(slide, level, x, y)
    if len(sys.argv) == 3:
        slide, kind = sys.argv[1:]
        return _legacy_one_shot_associated(slide, kind)
    print(
        "usage: oracle_runner.py <slide>                            # batched\n"
        "       oracle_runner.py <slide> <level> <x> <y>            # one-shot tile\n"
        "       oracle_runner.py <slide> <kind>                     # one-shot associated",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    sys.exit(main())
