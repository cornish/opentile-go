# Iris File Extension (IFE)

The IrisDigitalPathology project's bleeding-edge non-TIFF WSI container, designed for low-overhead pathology tile serving. File extension `.iris`. Format version 1.0; spec authored 2024–2025 by Iris Digital Pathology and shipped as the public v1.0 reference encoder/decoder in [`IrisDigitalPathology/Iris-File-Extension`](https://github.com/IrisDigitalPathology/Iris-File-Extension) (MIT). The format spec itself is CC BY-ND 4.0.

**v0.8 is the first non-TIFF format opentile-go reads.** It's also the first format opentile-go ships with no Python analogue at all — there's no tifffile-equivalent or upstream-opentile-equivalent for cross-validation; correctness rests on sample-tile SHA fixtures + synthetic-IFE-writer unit tests + per-fixture geometry pinning.

## Format basics

- **Container**: not TIFF. Custom binary layout — fixed-size FILE_HEADER (38 B) at offset 0, chained TILE_TABLE (44 B) → LAYER_EXTENTS (16-B header + 12-B entries) → TILE_OFFSETS (16-B header + 8-B entries). Tile bytes laid out contiguously between the structures and EOF.
- **Endianness**: little-endian on disk regardless of host.
- **Detection**: first 4 bytes equal `0x49726973` ("Iris" as LE-uint32). On disk: `0x73 0x69 0x72 0x49`. Implemented via `Factory.SupportsRaw(io.ReaderAt, int64) bool` — runs *before* `tiff.Open` in `opentile.Open`'s dispatch loop, so IFE files never get parsed as TIFF.
- **Layer ordering**: file stores layers **coarsest-first** (`scales` strictly increasing; first entry is the smallest, last entry is native). opentile-go's API exposes them **native-first** (`Levels()[0]` = highest resolution); the reader inverts the slice at parse time and never re-exposes the file's storage order.
- **Tile size**: hard-coded at 256×256 pixels per spec (not configurable per-file in v1.0). Image pixel dimensions at layer L: `(x_tiles[L] * 256, y_tiles[L] * 256)`. Right/bottom edge tiles may carry partial content but always full 256×256 framing.
- **Compression**: per-file (not per-tile). TILE_TABLE.encoding byte selects from JPEG (2), AVIF (3), or the Iris-proprietary codec (1); UNDEFINED (0) and unknown values are rejected at Open time. Tile bytes are **self-contained** — each is a complete JPEG / AVIF / IRIS bytestream. No JPEGTables splice (unlike SVS / BIF).
- **Sparse tiles**: a TILE_OFFSETS entry whose 40-bit offset field equals `NULL_TILE` (`0xFFFFFFFFFF`) indicates an absent tile — no compressed bytes on disk at that grid cell. opentile-go returns `(nil, ErrSparseTile)` wrapped in `TileError` so consumers can distinguish "no data here" from "I/O error" or "out of bounds." The cervix fixture has zero sparse tiles; synthetic tests cover the path.

## Fixture inventory

| File | Bytes | Encoding | Layers | Native dims (px) | Source |
|---|---:|---|---:|---|---|
| `cervix_2x_jpeg.iris` | 2.16 GB | JPEG | 9 | 126,976 × 88,576 | [Iris S3 public bucket](https://irisdigitalpathology.s3.us-east-2.amazonaws.com/example-slides/cervix_2x_jpeg.iris) |

Cervix is a **2× downsampled** export from the Iris reference encoder (full-res would be 253,952 × 177,152 px). It's the only public IFE fixture; locally-encoded fixtures via the user's separate Iris benchmarking workspace can be added later if needed.

## What's supported

| Capability | Status | Notes |
|---|---|---|
| Magic-byte detection (non-TIFF) | ✅ via `Factory.SupportsRaw` — first non-TIFF format opentile-go has shipped |
| FILE_HEADER + TILE_TABLE + LAYER_EXTENTS + TILE_OFFSETS parsing | ✅ | Pure stdlib `encoding/binary`; T1–T4 gate-verified against cervix |
| Layer-ordering inversion at parse time | ✅ | Reader builds file-order + API-order extent slices + a `layerCumulative` prefix-sum array; never exposes coarsest-first ordering across the public API |
| 9 pyramid levels exposed native-first | ✅ | All cervix levels pin against `tests/parity/ife_geometry_test.go` |
| Per-tile `ReadAt` lookup via 40+24-bit (offset, size) | ✅ | `Tile(col, row)` does prefix-sum lookup → tileOffsets[idx] → ReadAt; no decode |
| Sparse tiles via `ErrSparseTile` sentinel | ✅ | New v0.8 sentinel in `errors.go`; cervix has zero sparse entries — synthetic tests exercise the path |
| `TileAt(TileCoord)` 2D-only delegate | ✅ | Non-zero Z/C/T returns `ErrDimensionUnavailable` (matches the v0.7 multi-dim 2D-format pattern) |
| `TileReader` streaming | ✅ via `io.NewSectionReader` |
| `Tiles` iterator (row-major, serial) | ✅ |
| Three encoding values exposed via `Compression()` | ✅ | JPEG → `CompressionJPEG`; AVIF → `CompressionAVIF` (new); IRIS → `CompressionIRIS` (new) |
| Synthetic-writer test harness | ✅ in `formats/ife/synthetic_test.go` — covers layer inversion, sparse, IRIS / AVIF encodings, iterator order, error paths |

## What's not supported

| Capability | Status | Why |
|---|---|---|
| METADATA block parsing | ❌ deferred — L22 | Spec defines a METADATA block carrying vendor-specific properties + possibly associated images. v0.8 reads the offset but `Tiler.Metadata()` returns the zero value. Resolved when a consumer needs slide-level metadata |
| Annotations + attributes + associated images | ❌ deferred | All defined in IFE v1.0 but unused for the bench / tile-serving use cases. Add when a consumer surfaces them |
| Cipher block | ❌ ignored | Reserved for future Iris-Codec features; the reader expects `cipher_offset == NULL_OFFSET` (0xFFFFFFFFFFFFFFFF) and ignores the contents otherwise |
| AVIF tile decode | ❌ — consumer's call (L24) | opentile-go is a byte-passthrough library; linking libavif would expand the cgo footprint past `internal/jpegturbo/` and break the byte-passthrough contract. Consumer either ships libavif or `golang.org/x/image/avif` (when stdlib gains it) |
| Iris-proprietary codec decode | ❌ — consumer's call (L24) | Same as AVIF: passthrough only. opentile-go reports `CompressionIRIS` so consumers know they need an Iris codec; `Tile()` returns the raw bytes. Consumers that don't ship a codec typically 501 the request |
| Spec v2.0 fields | ❌ — error | v2.0 isn't out yet; the reader rejects `extension_major != 1` rather than silently misparsing future-format files |
| Cross-tool parity vs `tile_server_iris` | ❌ deferred — L23 | Would require a runner that shells out to the Iris HTTP server, similar shape to v0.7's openslide oracle but cross-language. Not load-bearing while the v0.8 sample-tile SHA fixtures + synthetic tests pass |

## Parity / correctness

IFE has **no Python or external-binary parity oracle**. v0.7's tifffile and Python-opentile oracles can't read IFE; openslide doesn't either. Coverage in v0.8 is layered:

1. **Sample-tile SHA fixtures** — `tests/fixtures/cervix_2x_jpeg.ife.json` (13.7 KB) committed via `TestGenerateFixtures`. `TestSlideParity` walks every level, samples ~10 tile positions per level via the seeded RNG harness, and SHA256-compares against the committed values. Locks in opentile-go's own output across regressions.
2. **Synthetic-IFE-writer unit tests** — `formats/ife/synthetic_test.go::synthBuilder` hand-rolls IFE byte buffers for known 1- to 3-layer fake slides with arbitrary recognizable tile bytes (`"NATIVE_LAYER_TILE_0_0"`, etc.). Drives the reader and asserts metadata + tile bytes match what the writer put in. Catches reader bugs without depending on the real fixture.
3. **Per-fixture geometry pinning** — `tests/parity/ife_geometry_test.go` asserts every level's Size + TileSize + Grid + the L0 tile's encoding-magic prefix (`ff d8` for JPEG cervix). No build tag; runs in `make test` whenever `OPENTILE_TESTDIR` is set.

The first cross-tool divergence we hit (opentile-go produces byte X, consumer Y observes byte Z) will be debugged from scratch. Acceptable risk for a bleeding-edge format; flagged in `CHANGELOG.md`'s [0.8.0] notes section.

## Deviations from upstream

The IFE v1.0 spec is the only upstream — there's no second implementation to deviate from in the way SVS / NDPI / Philips / OME / BIF deviate from Python opentile. **One observed file-vs-spec mismatch** captured in `docs/deferred.md` §1a:

| Deviation | Since | Opt-out | Reason |
|---|---|---|---|
| `TILE_TABLE.x_extent` / `y_extent` ignored for level dimensions | v0.8 | not opt-out-able | Spec doc claims these are "image width/height in pixels at top resolution layer." The cervix fixture stores `x_extent=496, y_extent=345` — exact matches for the native layer's `LAYER_EXTENTS.x_tiles=496, y_tiles≈346`, i.e. **tile counts**, not pixels. Reader derives image dims from `LAYER_EXTENTS.x_tiles × 256` instead. Either the spec doc is wrong or the cervix is non-conforming; without a second fixture we can't disambiguate. The `LAYER_EXTENTS` math is unambiguous either way |

**Architectural deviation** also captured in §1a: `FormatFactory.SupportsRaw` + `OpenRaw` non-TIFF dispatch path. Backward-compatible via embedded `RawUnsupported` defaults; no caller-visible breakage on the existing five TIFF formats.

## Implementation references

- Our package: `formats/ife/`.
- Public surface: `Factory` (exported via `formats/all`), `MagicBytes` constant, `NullTile` constant, `TileSidePixels` constant. Internal types (`FileHeader`, `TileTable`, `LayerExtent`, `TileEntry`) exported for advanced consumers but the Tiler / Level interfaces are the supported entry points.
- Spec doc: [`sample_files/ife/ife-format-spec-for-opentile-go.md`](../../sample_files/ife/ife-format-spec-for-opentile-go.md) — the project-internal byte-layout reference distilled from upstream (extracted 2026-04-28).
- Design doc: [`docs/superpowers/specs/2026-04-29-opentile-go-ife-design.md`](../superpowers/specs/2026-04-29-opentile-go-ife-design.md) — eight sealed decisions including the §3 dispatch refactor + §6 layer ordering + §7 parity strategy.
- Plan doc: [`docs/superpowers/plans/2026-04-29-opentile-go-v08-ife.md`](../superpowers/plans/2026-04-29-opentile-go-v08-ife.md) — 19 tasks across 5 batches.
- Upstream Iris-File-Extension: [github.com/IrisDigitalPathology/Iris-File-Extension](https://github.com/IrisDigitalPathology/Iris-File-Extension) (MIT). Headers in [Iris-Headers](https://github.com/IrisDigitalPathology/Iris-Headers).
- IFE v1.0 spec — bundled in the upstream repo's `docs/`. Spec text is CC BY-ND 4.0; opentile-go's reader is a clean-room port of the byte layout, not a derivative of the spec prose.

## Known issues + history

- **TILE_TABLE x_extent / y_extent** (T3 gate, v0.8): values match tile counts, not pixels. Reader ignores; `LAYER_EXTENTS` is canonical.
- **Self-contained tile bytes** (T8 implementation, v0.8): each tile is a complete JPEG / AVIF / IRIS bytestream. No JPEGTables splice — distinct from SVS / BIF abbreviated-scan pattern. Surfaced during smoke testing when the cervix tile bytes started with full `ff d8 ff e0 ... JFIF` prefixes rather than the SVS-style truncated SOS.

See [`docs/deferred.md`](../deferred.md) §8b (v0.8 retirement audit) for the full mid-task discovery log + per-task gate outcomes.
