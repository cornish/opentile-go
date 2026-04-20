# opentile-go

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

Pure-Go port of [opentile](https://github.com/imi-bigpicture/opentile), a library for reading
tiles from whole-slide imaging (WSI) TIFF files used in digital pathology.

**Status — v0.1**: Aperio SVS is supported (JPEG and JPEG 2000 tiles, pass-through).
NDPI, Philips, 3DHistech, OME TIFF are on the roadmap. Associated images
(label, overview, thumbnail) are deferred to v0.3. See
[`docs/deferred.md`](./docs/deferred.md) for the full roadmap and known limitations.

## Install

```
go get github.com/tcornish/opentile-go
```

Requires Go 1.23+. No cgo, no C dependencies.

## Usage

```go
package main

import (
    "fmt"
    "log"

    opentile "github.com/tcornish/opentile-go"
    _ "github.com/tcornish/opentile-go/formats/all"
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
import svs "github.com/tcornish/opentile-go/formats/svs"

md := tiler.Metadata()
fmt.Println("magnification:", md.Magnification)

if sm, ok := svs.MetadataOf(tiler); ok {
    fmt.Println("MPP:", sm.MPP, "µm/px")
}
```

## Concurrency

`Level.Tile(x, y)` and `Level.TileReader(x, y)` are safe to call concurrently from multiple goroutines, provided the underlying `io.ReaderAt` supplied to `Open` is also safe for concurrent use. `*os.File` satisfies this, so `OpenFile` is goroutine-safe out of the box. All internal caches (parsed IFDs, per-tile offset/length tables, metadata) are populated at `Open()` time and then immutable — no locks on the tile hot path.

`Close()` must not race with in-flight tile reads. Drain before closing.

## Testing

```
go test ./... -race
```

Integration tests require real slide files at `$OPENTILE_TESTDIR`. The three fixtures committed to `tests/fixtures/` were generated against openslide's public testdata:

```
OPENTILE_TESTDIR="$PWD/sample_files/svs" go test ./tests/... -v
```

To regenerate fixtures from fresh slides:

```
OPENTILE_TESTDIR="$PWD/sample_files/svs" \
    go test ./tests -tags generate -run TestGenerateFixtures -generate -v
```

## Scope

See [`docs/superpowers/specs/2026-04-19-opentile-go-design.md`](./docs/superpowers/specs/2026-04-19-opentile-go-design.md) for the full design and non-goals, and [`docs/deferred.md`](./docs/deferred.md) for the roadmap and known v0.1 limitations.

## License

Apache 2.0. This is an independent Go port of the Python `opentile` library (Copyright 2021–2024 Sectra AB); see [NOTICE](./NOTICE) for full attribution. Not affiliated with or endorsed by Sectra AB or the BigPicture project.
