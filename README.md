# opentile-go

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

A Go library for reading raw compressed tiles from whole-slide imaging (WSI) files used in digital pathology, including TIFF dialects (Aperio SVS, Hamamatsu NDPI, Philips TIFF, OME-TIFF, Ventana BIF) and the bleeding-edge non-TIFF [Iris File Extension](https://github.com/IrisDigitalPathology/Iris-File-Extension). Direct port of the Python [opentile](https://github.com/imi-bigpicture/opentile) library for the four TIFF formats it supports, with byte-identical output. BIF (v0.7) and IFE (v0.8) are opentile-go's own additions beyond upstream's coverage.

```go
import (
    opentile "github.com/cornish/opentile-go"
    _ "github.com/cornish/opentile-go/formats/all"
)

t, err := opentile.OpenFile("slide.svs")
if err != nil { /* ... */ }
defer t.Close()

base, _ := t.Level(0)
tile, err := base.Tile(0, 0) // raw compressed JPEG / JP2K / etc. bytes
```

`Tile(x, y)` returns the raw compressed bitstream as stored on disk — opentile-go is a tile-extraction library, not a decoder. Decode the returned bytes with whatever JPEG / JPEG 2000 / etc. library suits your downstream pipeline.

## Supported formats

| Format | Extension | Levels | Associated | Compression | Parity bar | Detail |
|---|---|---|---|---|---|---|
| **Aperio SVS** | `.svs` | tiled | label, overview, thumbnail | JPEG, JP2K (passthrough) | byte-parity vs. Python opentile | [docs/formats/svs.md](./docs/formats/svs.md) |
| **Hamamatsu NDPI** | `.ndpi` | tiled (striped + OneFrame) | overview, synthesised label\*, Map\* | JPEG | byte-parity vs. Python opentile | [docs/formats/ndpi.md](./docs/formats/ndpi.md) |
| **Philips TIFF** | `.tiff` | tiled, with sparse-tile fill | label, overview, thumbnail | JPEG | byte-parity vs. Python opentile | [docs/formats/philips.md](./docs/formats/philips.md) |
| **OME-TIFF** | `.ome.tiff` | tiled (SubIFD) + OneFrame | macro, label, thumbnail | JPEG (uint8 RGB only) | byte-parity vs. Python opentile + tifffile | [docs/formats/ome.md](./docs/formats/ome.md) |
| **Ventana BIF** | `.bif` | tiled, serpentine remap, with overlap metadata\* + ScanWhitePoint blank-tile fill | overview, probability\*, thumbnail | JPEG | tifffile (DP 200) + sampled-tile SHAs (both fixtures) | [docs/formats/bif.md](./docs/formats/bif.md) |
| **Iris IFE\*** | `.iris` | tiled (256×256, native-first inversion) with sparse-tile sentinel | none in v0.8 | JPEG, AVIF (passthrough), Iris-proprietary (passthrough) | sampled-tile SHAs + synthetic-writer + per-fixture geometry pin | [docs/formats/ife.md](./docs/formats/ife.md) |

\* Marks Go-side extensions beyond upstream Python opentile; see [Deviations](#deviations-from-upstream-python-opentile) below.

**Detection** is automatic. `opentile.OpenFile` walks the registered factories — first asking each for `SupportsRaw(r, size)` against the raw byte stream, then falling through to TIFF-parsed `Supports(file)` — and dispatches the first match. The two-stage dispatch lets non-TIFF formats (IFE) short-circuit before `tiff.Open`. Format packages register at import time via `_ "github.com/cornish/opentile-go/formats/all"`.

**Format coverage**: opentile-go ports the four TIFF formats Python opentile 0.20.0 supports for tile extraction. 3DHistech TIFF (the fifth upstream format) is parked at [#2](https://github.com/cornish/opentile-go/issues/2). Ventana BIF — the first beyond upstream's coverage — landed in v0.7. Iris IFE — the first non-TIFF format — landed in v0.8. Sakura SVSlide is parked at [#3](https://github.com/cornish/opentile-go/issues/3).

## Prerequisites

- **Go 1.23+** (uses `iter.Seq2`).
- **libjpeg-turbo 2.1+** for tile-domain JPEG operations (NDPI edge-tile fill, Philips sparse-tile fill, OME OneFrame extraction).
  - macOS: `brew install jpeg-turbo`
  - Debian / Ubuntu: `apt-get install libturbojpeg0-dev`
- **`pkg-config`** to resolve `libturbojpeg` at build time.

opentile-go is **mostly Go with one cgo dependency** — `internal/jpegturbo/` wraps libjpeg-turbo's `tjTransform` for lossless DCT-domain crops. Building without cgo (`-tags nocgo` or `CGO_ENABLED=0`) is supported: SVS works fully, NDPI striped levels work, but NDPI OneFrame / NDPI edge-tile fill / Philips sparse-tile fill / OME OneFrame return `ErrCGORequired`.

## Install

```sh
go get github.com/cornish/opentile-go
```

Pin to v0.5.1 or later (v0.5.0 shipped with a wrong module path; see [CHANGELOG](./CHANGELOG.md)).

## API

### Opening a slide

```go
t, err := opentile.OpenFile("slide.tiff")
if err != nil { /* ErrUnsupportedFormat or open error */ }
defer t.Close()

fmt.Println("format:", t.Format())                 // "svs", "ndpi", "philips", "ome", "bif", "ife"
fmt.Println("levels:", len(t.Levels()))
```

Pass options to override defaults:

```go
t, err := opentile.OpenFile("slide.ndpi",
    opentile.WithTileSize(1024, 1024),                     // virtual tile size for OneFrame levels
    opentile.WithNDPISynthesizedLabel(false),              // disable the v0.2 NDPI label synthesis
)
```

For an `io.ReaderAt` source (S3, in-memory, etc.) instead of a filename:

```go
t, err := opentile.Open(reader, size, opts...)
```

### Reading tiles

```go
base, _ := t.Level(0)

// Per-tile metadata.
fmt.Printf("base: %v tiles of %v pixels, compression %s, mpp %v\n",
    base.Grid(), base.TileSize(), base.Compression(), base.MPP())

// Get one tile's raw compressed bytes.
tile, err := base.Tile(0, 0)
```

Stream a tile via `io.ReadCloser`:

```go
rc, err := base.TileReader(0, 0)
defer rc.Close()
io.Copy(dst, rc)
```

Iterate every tile in row-major order:

```go
for pos, res := range base.Tiles(ctx) {
    if res.Err != nil { /* ... */ }
    process(pos.X, pos.Y, res.Bytes)
}
```

### Multi-image files

OME-TIFF can carry multiple main pyramids in a single file. `Tiler.Images()` returns them all; `Tiler.Levels()` is a shortcut to `Images()[0].Levels()` for callers that don't need to distinguish.

```go
for _, img := range t.Images() {
    fmt.Printf("Image %d (%q): %d levels, %v µm/px\n",
        img.Index(), img.Name(), len(img.Levels()), img.MPP())
    base, _ := img.Level(0)
    tile, _ := base.Tile(0, 0)
    // ...
}
```

For SVS, NDPI, and Philips, `Images()` always returns a one-element slice — Levels() / Level(i) work as before.

### Associated images

`Tiler.Associated()` returns label / overview / thumbnail / map images where the format provides them:

```go
for _, a := range t.Associated() {
    b, err := a.Bytes()
    if err != nil { continue }
    fmt.Printf("%s: %v, %s, %d bytes\n", a.Kind(), a.Size(), a.Compression(), len(b))
}
```

`a.Bytes()` returns a self-contained, decoder-ready blob in whatever codec the source TIFF carries (typically JPEG or LZW). `a.Kind()` is `"label"`, `"overview"`, `"thumbnail"`, or `"map"` (NDPI only).

### Format-specific metadata

Cross-format fields (manufacturer, scanner serial, acquisition datetime, magnification) are surfaced via `t.Metadata()`. Format-specific fields are accessible by type-asserting through a per-format helper:

```go
import (
    svs "github.com/cornish/opentile-go/formats/svs"
    ndpi "github.com/cornish/opentile-go/formats/ndpi"
    philips "github.com/cornish/opentile-go/formats/philips"
    ome "github.com/cornish/opentile-go/formats/ome"
)

if md, ok := svs.MetadataOf(t); ok {
    fmt.Println("MPP (SVS):", md.MPP, "µm/px")
}
if md, ok := ndpi.MetadataOf(t); ok {
    fmt.Println("source lens (NDPI):", md.SourceLens, "x")
}
if md, ok := philips.MetadataOf(t); ok {
    fmt.Println("PixelSpacing (Philips):", md.PixelSpacing, "mm")
}
if md, ok := ome.MetadataOf(t); ok {
    fmt.Println("OME images:", len(md.Images))
}
```

`MetadataOf` walks any number of wrapper Tilers (e.g., `*fileCloser` from `OpenFile`) before asserting on the concrete type, so the helper works regardless of how the Tiler was obtained.

### Concurrency

`Level.Tile(x, y)` and `Level.TileReader(x, y)` are safe to call concurrently from multiple goroutines, provided the `io.ReaderAt` passed to `Open` is concurrent-safe. `*os.File` satisfies this; `OpenFile` is goroutine-safe out of the box.

All internal caches (parsed IFDs, per-tile offset / length tables, metadata) are populated at `Open()` time and then immutable — no locks on the tile hot path. Format packages with shared lazy caches (`formats/ndpi/striped.go::Tile`'s per-frame assembly cache; `formats/ndpi/oneframe.go`'s extended-frame cache; `formats/philips/tiled.go`'s blank-tile cache) use `sync.Once` and produce byte-deterministic output regardless of which goroutine populates them first.

`Close()` must not race with in-flight tile reads — drain before closing.

## Deviations from upstream Python opentile

opentile-go aims for byte-parity with Python opentile 0.20.0. A small number of deviations exist where matching upstream would encode an upstream oversight or where opentile-go provides a strictly more useful affordance:

| Deviation | Format | Since | Opt-out / API | Why |
|---|---|---|---|---|
| Synthesised label | NDPI | v0.2 | `WithNDPISynthesizedLabel(false)` | Upstream doesn't surface NDPI labels at all; we crop the left 30% of the overview to provide an Aperio-style label affordance. |
| Map pages exposed | NDPI | v0.4 | not opt-out-able (silent absence) | tifffile already classifies them as `series.name == 'Map'`; surfacing matches the underlying TIFF carrying. |
| Multi-image OME pyramids | OME | v0.6 | use `Tiler.Levels()` instead of `Tiler.Images()` for first-image-only behaviour | Upstream's base Tiler loop silently drops 3 of 4 main pyramids in multi-image files via an unintentional last-wins assignment. We expose all of them via `Tiler.Images()`. |
| Probability map exposed as `kind="probability"` | BIF | v0.7 | iterate `Associated()` and skip the kind | Upstream doesn't read BIF; openslide drops the probability map. We surface it for downstream tools that want it. |
| `Level.TileOverlap() image.Point` interface evolution | BIF + all | v0.7 | non-BIF formats return `image.Point{}` (zero) — no caller change needed | BIF level-0 stores tiles with horizontal overlap; consumer needs the value to position raw tile bytes correctly. |
| Non-strict `ScannerModel` acceptance | BIF | v0.7 | not opt-out-able | The BIF spec mandates rejecting any slide whose `ScannerModel != "VENTANA DP 200"`; we accept any iScan-tagged BigTIFF and route via `HasPrefix("VENTANA DP")` so legacy iScan slides aren't worse-than-openslide. |
| Multi-dimensional WSI API addition (`TileCoord` + `Level.TileAt` + `Image.SizeZ/SizeC/SizeT/ChannelName/ZPlaneFocus`) | All formats | v0.7 | additive — 2D-only formats inherit `SingleImage` defaults | Modern WSI consumers (fluorescence, focal-plane viewers, time series) need explicit multi-dim addressing. BIF reads multi-Z natively; OME surfaces dimensions honestly + defers `TileAt(z != 0)` to a future format-package milestone. |
| Non-TIFF dispatch path (`FormatFactory.SupportsRaw` + `OpenRaw` + `RawUnsupported` base) | All formats | v0.8 | additive — TIFF factories embed `RawUnsupported` and inherit defaults | Iris IFE is the first non-TIFF format opentile-go reads. Table-driven dispatch lets each format own its detection; future non-TIFF formats drop in additively. |
| `TILE_TABLE.x_extent` / `y_extent` ignored | IFE | v0.8 | not opt-out-able | The IFE v1.0 spec doc claims these fields carry image pixel dims, but the cervix fixture stores tile counts (matching `LAYER_EXTENTS.x_tiles`). Reader derives image dims from `LAYER_EXTENTS × 256` instead — unambiguous either way. |

Full reasoning + per-deviation commit references are in [`docs/deferred.md`](./docs/deferred.md).

## Testing

```sh
make test     # go test ./... -race -count=1
make vet      # go vet ./...
make cover    # ≥80% per package; needs OPENTILE_TESTDIR
make parity   # batched parity oracle vs Python opentile 0.20.0 + tifffile
make bench    # NDPI per-tile throughput regression gate
```

Integration tests and the parity oracle require real slide files at `$OPENTILE_TESTDIR`. Fixture JSONs (committed) are at `tests/fixtures/`. Slides themselves are not redistributable and are gitignored.

```sh
OPENTILE_TESTDIR="$PWD/sample_files" go test ./tests/... -v
```

For parity testing against Python opentile + tifffile, set the Python interpreter and run with the `parity` build tag:

```sh
pip install -r tests/oracle/requirements.txt
OPENTILE_ORACLE_PYTHON=$(which python) \
OPENTILE_TESTDIR="$PWD/sample_files" \
  go test ./tests/oracle/... -tags parity -v
```

The default run samples ~100 tile positions per level per slide. A persistent stdin / stdout protocol keeps one Python subprocess resident per slide; full sweep on the v0.6 13-slide oracle slate completes in under 10 seconds.

## License + attribution

Apache 2.0. Independent Go port of the Python [opentile](https://github.com/imi-bigpicture/opentile) library (Copyright 2021–2024 Sectra AB); see [NOTICE](./NOTICE) for attribution. Not affiliated with or endorsed by Sectra AB or the BigPicture project.
