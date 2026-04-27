# opentile-go

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

Pure-Go port of [opentile](https://github.com/imi-bigpicture/opentile), a library for reading
tiles from whole-slide imaging (WSI) TIFF files used in digital pathology.

**Status — v0.5**: Philips TIFF added — third format opentile-go handles,
paralleling the v0.2 NDPI add. Aperio SVS (JPEG and JPEG 2000), Hamamatsu
NDPI, and Philips TIFF fully supported, with associated images
(label, overview, thumbnail, NDPI Map pages), BigTIFF, Hamamatsu's
64-bit offset extension, and the Philips sparse-tile blank-tile
mechanism. Public API frozen since v0.3.

Output is **byte-identical to Python
[opentile](https://github.com/imi-bigpicture/opentile) 0.20.0 on every
sampled tile and associated image we expose**, across all 11 oracle
slides (5 SVS + 2 NDPI + 4 Philips); 12 fixtures total in the
integration suite (the 11 above plus Hamamatsu-1.ndpi sampled). The
v0.4 NDPI edge-tile divergence (was tracked as L12) was diagnosed as a
control-flow bug in our Go-side dispatch and fixed in v0.4 —
geometry-first dispatch matches Python's `__need_fill_background` gate
exactly.

Three permanent design choices remain documented (L4 missing-MPP, L5
NDPI sniff, L14 Go-side label synthesis on NDPI). Aperio SVS
corrupt-edge reconstruct (R4) and JPEG 2000 decode/encode (R9) are
parked at [#1](https://github.com/cornish/opentile-go/issues/1) until
a real slide motivates them.

OME TIFF is next (v0.6); after that opentile-go ventures beyond
upstream's coverage starting with Ventana BIF (v0.7). 3DHistech TIFF
and Sakura SVSlide are parked. See
[`docs/deferred.md`](./docs/deferred.md) for the full roadmap.

## Prerequisites

- Go 1.23+ (for `iter.Seq2`)
- [libjpeg-turbo](https://libjpeg-turbo.org/) 2.1+ (for NDPI pyramid levels and
  NDPI label cropping)
  - macOS: `brew install jpeg-turbo`
  - Debian/Ubuntu: `apt-get install libturbojpeg0-dev`
- `pkg-config` to resolve `libturbojpeg` at build time

Building without cgo is supported via `-tags nocgo` (or `CGO_ENABLED=0`). The
nocgo build supports SVS (all features) and NDPI striped pyramid levels; NDPI
one-frame pyramid levels and NDPI edge-tile fill return `ErrCGORequired`.

## Install

```
go get github.com/cornish/opentile-go
```

## Usage

```go
package main

import (
    "fmt"
    "log"

    opentile "github.com/cornish/opentile-go"
    _ "github.com/cornish/opentile-go/formats/all"
)

func main() {
    tiler, err := opentile.OpenFile("slide.svs")
    if err != nil {
        log.Fatal(err)
    }
    defer tiler.Close()

    fmt.Println("format:", tiler.Format())
    fmt.Println("levels:", len(tiler.Levels()))

    base, _ := tiler.Level(0)
    fmt.Printf("base: %v tiles of %v pixels, compression %s\n",
        base.Grid(), base.TileSize(), base.Compression())

    tile, err := base.Tile(0, 0)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("tile[0,0]: %d bytes of %s\n", len(tile), base.Compression())
}
```

`Tile(x, y)` returns the raw compressed bitstream exactly as stored in the source TIFF (JPEG or JPEG 2000 in v0.1). Decode with any codec appropriate for the reported `Compression`.

### Streaming

For memory-sensitive callers, `Level.TileReader(x, y)` returns an `io.ReadCloser` backed by an `io.SectionReader`, avoiding a buffer copy:

```go
rc, err := base.TileReader(0, 0)
if err != nil { log.Fatal(err) }
defer rc.Close()
_, _ = io.Copy(dst, rc)
```

### Iteration

`Level.Tiles(ctx)` yields every tile position in row-major order as a Go 1.23 iterator:

```go
for pos, res := range base.Tiles(ctx) {
    if res.Err != nil { /* ... */ }
    process(pos.X, pos.Y, res.Bytes)
}
```

### Metadata

`Tiler.Metadata()` returns the cross-format fields — magnification, scanner, acquisition datetime. Aperio-specific fields (MPP, software line, filename) are accessible via `svs.MetadataOf`:

```go
import svs "github.com/cornish/opentile-go/formats/svs"

md := tiler.Metadata()
fmt.Println("magnification:", md.Magnification)

if sm, ok := svs.MetadataOf(tiler); ok {
    fmt.Println("MPP:", sm.MPP, "µm/px")
}
```

NDPI-specific fields (source-lens magnification, focal offset, scanner serial) are accessible via `ndpi.MetadataOf`:

```go
import ndpi "github.com/cornish/opentile-go/formats/ndpi"

if nm, ok := ndpi.MetadataOf(tiler); ok {
    fmt.Println("source lens:", nm.SourceLens, "x")
    fmt.Println("focal offset:", nm.FocalOffset, "mm")
    fmt.Println("scanner:", nm.Reference)
}
```

### Associated images

`Tiler.Associated()` returns label / overview / thumbnail images where the format provides them. Each `AssociatedImage` exposes `Kind()`, `Size()`, `Compression()`, and `Bytes()` (a standalone, decoder-ready blob in whatever codec the source TIFF carries):

```go
for _, a := range tiler.Associated() {
    b, err := a.Bytes()
    if err != nil { continue }
    fmt.Printf("%s: %v, %s, %d bytes\n", a.Kind(), a.Size(), a.Compression(), len(b))
}
```

SVS slides provide thumbnail, overview, and label (the label is emitted as raw LZW strip-0 bytes matching upstream Python opentile's behavior — see `docs/deferred.md` L10). NDPI slides provide overview and a synthesized label cropped from the overview's left 30%.

## Concurrency

`Level.Tile(x, y)` and `Level.TileReader(x, y)` are safe to call concurrently from multiple goroutines, provided the underlying `io.ReaderAt` supplied to `Open` is also safe for concurrent use. `*os.File` satisfies this, so `OpenFile` is goroutine-safe out of the box. All internal caches (parsed IFDs, per-tile offset/length tables, metadata) are populated at `Open()` time and then immutable — no locks on the tile hot path.

`Close()` must not race with in-flight tile reads. Drain before closing.

## Testing

```
go test ./... -race
```

Integration tests require real slide files at `$OPENTILE_TESTDIR`. Fixtures
for five slides are committed to `tests/fixtures/` (three SVS from openslide's
public testdata plus two NDPI — CMU-1 and OS-2). The harness walks both
`svs/` and `ndpi/` subdirectories of `$OPENTILE_TESTDIR`:

```
OPENTILE_TESTDIR="$PWD/sample_files" go test ./tests/... -v
```

To regenerate fixtures from fresh slides:

```
OPENTILE_TESTDIR="$PWD/sample_files" \
    go test ./tests -tags generate -run TestGenerateFixtures -generate -v
```

### Parity testing against Python opentile

An opt-in `//go:build parity` harness byte-compares tile and associated-image
output against Python [opentile](https://github.com/imi-bigpicture/opentile)
0.20.0, the reference implementation. The Go side shells out to a Python
subprocess (`tests/oracle/oracle_runner.py`) for each tile or associated
image and compares bytes:

```
pip install -r tests/oracle/requirements.txt
OPENTILE_ORACLE_PYTHON=$(which python) \
OPENTILE_TESTDIR="$PWD/sample_files" \
  go test ./tests/oracle/... -tags parity -v
```

Set `OPENTILE_ORACLE_PYTHON` to point at the interpreter that has opentile
installed (typically a venv). The default run samples ~100 tile positions per
level per slide (corners + diagonals + a 10×10 stride fill); pass
`-parity-full` to walk every tile (adds minutes to tens of minutes per slide).
A persistent stdin/stdout protocol keeps one Python subprocess resident per
slide rather than spawning one per request, so the default sweep on all 7
oracle slides completes in under 10 seconds on an M-series Mac.

The harness reports byte-identical output on all sampled tiles for all
committed fixtures, with the L12 edge-tile divergence on NDPI downgraded to
`t.Log` (see `docs/deferred.md`).

### Test helpers

Test helpers (config builders, fixture types) live in the
[`opentiletest`](./opentile/opentiletest) sibling package, mirroring the stdlib
idiom (`httptest`, `iotest`):

```go
import "github.com/cornish/opentile-go/opentile/opentiletest"

cfg := opentiletest.NewConfig(opentile.Size{W: 512, H: 512}, opentile.CorruptTileError)
```

## Scope

See [`docs/superpowers/specs/2026-04-24-opentile-go-v03-design.md`](./docs/superpowers/specs/2026-04-24-opentile-go-v03-design.md) for the v0.3 design and the [`docs/deferred.md`](./docs/deferred.md) for the active roadmap and known limitations. Earlier milestone specs are kept under `docs/superpowers/specs/` for reference (v0.1: 2026-04-19, v0.2: 2026-04-21).

## License

Apache 2.0. This is an independent Go port of the Python `opentile` library (Copyright 2021–2024 Sectra AB); see [NOTICE](./NOTICE) for full attribution. Not affiliated with or endorsed by Sectra AB or the BigPicture project.
