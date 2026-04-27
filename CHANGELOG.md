# Changelog

All notable changes to opentile-go are recorded here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) loosely;
versioning is semantic (`MAJOR.MINOR.PATCH`).

The single source of truth for "what was deferred and why" is
[`docs/deferred.md`](docs/deferred.md). This file is the curated
front-page summary; the deferred file has the full reasoning,
upstream references, and retirement audit per milestone.

## [Unreleased]

Active limitations after v0.5 are exclusively Permanent design choices
(L4, L5, L14 in `docs/deferred.md` §2). Open work parked in tracked
issues:

- **R4 / R9** ([#1](https://github.com/cornish/opentile-go/issues/1)) —
  SVS corrupt-edge reconstruct + JP2K decode/encode. Deferred from v0.4
  to v0.5+; no local SVS slide exhibits the corrupt-edge bug, so the
  work is parked until a real slide motivates it.

## [0.5.1] — 2026-04-26

### Fixed

- **Module path** — `go.mod` and every Go import statement renamed
  from `github.com/tcornish/opentile-go` to `github.com/cornish/opentile-go`,
  matching the actual GitHub repo location. v0.5.0's module path was
  wrong and `go get github.com/cornish/opentile-go@v0.5.0` failed for
  downstream consumers; pin to v0.5.1 or later. No public API changes;
  purely a packaging fix.

## [0.5.0] — 2026-04-26

Philips TIFF support — the third format opentile-go handles, paralleling
the v0.2 NDPI add. Output is **byte-identical to Python opentile
0.20.0** on every sampled tile and every associated image we expose,
across all 11 oracle slides (5 SVS + 2 NDPI + 4 Philips).

### Added

- **Philips TIFF format** — pyramid levels with sparse-tile
  blank-tile filling, label / macro / thumbnail associated images,
  DICOM-XML metadata extraction. Surface area: `formats/philips.New()`
  factory (registered by `formats/all`), `philips.MetadataOf(tiler)`
  for format-specific fields (PixelSpacing, BitsAllocated, etc.).
  4 sample fixtures (`Philips-{1,2,3,4}.tiff`, 277 MB to 3.1 GB; one
  is BigTIFF) in the integration + parity slates.
- `opentile.FormatPhilips` constant.
- `internal/jpegturbo.FillFrame` — new cgo entry point. tjTransform
  with an all-blocks CUSTOMFILTER overwriting every DCT coefficient
  to a luminance fill (DC = `LuminanceToDCCoefficient(luminance)`,
  AC = 0 on luma; chroma fully zeroed). Mirrors Python opentile's
  `JpegFiller.fill_image`. Used by Philips's sparse-tile blank-tile
  derivation.
- `internal/jpeg.InsertTables` — JPEGTables splice without APP14,
  sibling to `InsertTablesAndAPP14` used by SVS. Philips encodes
  standard YCbCr so no Adobe APP14 marker is needed.
- `internal/tiff.TagSoftware` constant + `Page.Software()` accessor
  (TIFF tag 305) used by Philips detection.

### Architecture

- DICOM-XML parsing via stdlib `encoding/xml` — first new use of
  the package in the codebase. Stack-based token decoder mirrors
  `ElementTree.iter('Attribute')`, descending into nested
  `<PIM_DP_SCANNED_IMAGES><Array><DataObject>...` wrappers that
  carry per-level Attributes in real fixtures.
- Per-level dimension correction via `formats/philips/dimensions.go`
  — direct port of `tifffile._philips_load_pages`. The first
  `DICOM_PIXEL_SPACING` entry calibrates the baseline mm scale; each
  subsequent entry produces a corrected size for the next tiled
  page, replacing the on-disk placeholder dimensions.
- Tile grid uses CORRECTED dims, not on-disk dims, matching Python's
  `image_size.ceil_div(tile_size)`. On-disk pages may carry more
  tile entries than `gx*gy`; trailing entries are unused but
  preserved for index parity with Python's
  `_tile_point_to_frame_index`.
- Sparse-tile blank tile is computed lazily on first sparse access
  (`sync.Once`); seed = first non-zero `TileByteCounts` entry, run
  through `InsertTables` → `FillFrame(luminance=1.0)`. Output
  byte-identical to Python's `Jpeg.fill_frame` on the same input.

### Retired

- **R5** (Philips TIFF) — landed end-to-end. `docs/deferred.md §7`
  has the v0.5 retirement audit + the three JIT-gate outcomes
  (T1 detection, T2 FillFrame determinism, T3 DICOM XML schema).

## [0.4.0] — 2026-04-26

NDPI completeness milestone. Output is **byte-identical to Python
opentile 0.20.0** on every sampled tile and every associated image we
expose, across all 7 fixtures in the parity oracle.

### Fixed

- **L12** — NDPI edge-tile OOB fill. Was misframed in v0.2 / v0.3 as
  "tjTransform CUSTOMFILTER non-determinism"; root cause re-diagnosed
  as a control-flow bug in `formats/ndpi/striped.go::Tile`. Pre-v0.4
  tried plain `Crop` first and silently returned mid-gray OOB fills
  (DC=0) on tiles where Crop succeeded despite extending past the
  image. Fix: dispatch geometry-first against image size, matching
  Python's `__need_fill_background` gate
  (`turbojpeg.py:839-863`). CMU-1 / OS-2 / Hamamatsu-1 NDPI fixtures
  regenerated; parity oracle's L12 `t.Logf` carve-out removed.
- **L17** — NDPI label `cropH` passes the full image height now,
  matching Python's `_crop_parameters[3] = page.shape[0]`. Pre-v0.4
  we floored the height to a whole-MCU multiple, dropping the last
  partial-MCU row. The pre-v0.4 deferred entry's "needs
  CropWithBackground" advice was wrong — libjpeg-turbo's
  `TJXOPT_PERFECT` accepts the partial last MCU row when the crop
  ends at the image edge.

### Added

- **L6 / R13** — NDPI Map pages (Magnification == -2.0) now surface
  as `AssociatedImage` entries with `Kind() == "map"`. Deliberate
  Go-side extension paralleling the v0.2 NDPI synthesised label
  (L14): upstream Python opentile chose not to surface Map pages
  even though tifffile classifies them as `series.name == 'Map'`
  one layer below.

### Deferred

- **R4** (SVS corrupt-edge reconstruct) and **R9** (JPEG 2000
  decode/encode) parked at
  [#1](https://github.com/cornish/opentile-go/issues/1). None of
  our 5 local SVS slides exhibits the corrupt-edge bug; 12 tasks
  of new cgo (libopenjp2 + jpegturbo Decode/Encode) plus a Pillow
  byte-equivalent BILINEAR port plus reconstruct.go for a
  synthetic-fixture-only feature is speculation, not completeness.
  Issue captures the full upstream algorithm, dependency tree,
  byte-parity bar from the v0.4 T1 determinism gate, and trigger
  conditions.

## [0.3.0] — 2026-04-25

Polish milestone over v0.2. Closes the v0.2 review surface (16
limitations + 25+ reviewer suggestions). **Public API frozen** from
this point — every name in `go doc ./...` survives v0.3 → v0.4
unchanged unless explicitly versioned.

### Added

- `ErrTooManyIFDs` sentinel error (A1).
- `Formats() []Format` introspection helper (A3).
- `WithNDPISynthesizedLabel(bool)` opt-out for the Go-side NDPI label
  synthesis (N-5).
- `OpenFile` errors now include the path (A2).
- `Config.TileSize` zero-size semantics documented (A4).
- `opentile/opentiletest/` sibling package for test helpers, mirroring
  stdlib's `httptest` / `iotest` idiom (T1).
- New SVS fixtures: `scan_620_.svs` (270 MB Grundium full-walk) and
  `svs_40x_bigtiff.svs` (4.8 GB Grundium sampled).
- `Makefile` with `test`, `cover`, `parity`, `vet`, `bench` targets.
- `make cover` gate enforcing ≥80% coverage per package.

### Changed

- **Batched parity oracle runner** — one Python subprocess per slide
  rather than per request. Default sample raised from ~10 to ~100
  positions per level; full sweep on all 7 oracle slides is now
  under 10 seconds (~10× faster than v0.2).
- SVS classifier now ports tifffile's `_series_svs` algorithm
  (replaces v0.2's positional one).
- `internal/tiff/walkIFDs` bulk-reads each IFD body in one ReadAt,
  ~2-4× faster on multi-page slides (O1).

### Fixed

- **L1** — SVS `SoftwareLine` had a trailing `\r` (CRLF parsing
  fix in `formats/svs/metadata.go`).
- **L7 + L11** — derive MCU size from SOF instead of hardcoding
  16×16 across NDPI overview crop and SVS associated-image DRI.
- **L10** — SVS LZW label was returning only strip 0 of multi-strip
  labels; now decodes all strips, raster-concatenates, and
  re-encodes as a single LZW stream.
- **L18** — `ConcatenateScans` rejected `ColorspaceFix=true` when
  `JPEGTables` was empty; matches Python's gate now (skip splice +
  APP14 when tables absent — required for Grundium SVS).
- BigTIFF tile offsets widened to uint64 (was rejecting
  `unsupported type 16`).
- `ConcatenateScans` dropped EOI assertions to match upstream's
  unconditional `frame[-2:] = end_of_image()` overwrite.

### Documented (no behaviour change)

- D1 — `decodeASCII` NUL-terminator tolerance.
- D2 — `decodeInline` `*byteReader` rationale.
- D3 — `Metadata.AcquisitionDateTime` `IsZero()` sentinel.
- I2 — `walkIFDs` overlapping-IFD detection limit.
- I7 — `ReplaceSOFDimensions` byte-scan invariant.
- N-6 — `CropWithBackground` chroma-DC=0 visual behaviour.
- N-9 — NDPI sniff cross-cutting peek rationale.
- O2 — `int(e.Count)` 32-bit truncation note.

## [0.2.0] — 2026-04-21

Second functional milestone. Adds NDPI support (the second WSI
format), BigTIFF, associated images on both formats, and the Python
parity oracle infrastructure that has guided every release since.

### Added

- **Hamamatsu NDPI format** — striped + one-frame pyramid levels,
  including the 64-bit offset extension for slides > 4 GB.
  Associated images (overview + synthesised label).
  `ndpi.MetadataOf` for source-lens / focal-offset / scanner serial.
- **BigTIFF** support across `internal/tiff`, transparent to format
  packages.
- **SVS associated images** — label, overview, thumbnail surfaced
  via `Tiler.Associated()`.
- **`internal/jpeg`** — pure-Go marker library with `ConcatenateScans`
  byte-identical to Python opentile's `jpeg.concatenate_scans`,
  plus `InsertTablesAndAPP14`, `NDPIStripeJPEGHeader`, `LumaDCQuant`
  / `LuminanceToDCCoefficient`.
- **`internal/jpegturbo`** — cgo wrapper over libjpeg-turbo for
  lossless MCU-aligned crop with CUSTOMFILTER-driven white-fill OOB
  for edge tiles. Builds without cgo via `-tags nocgo` (returns
  `ErrCGORequired`).
- **Python parity oracle** under `//go:build parity`
  (`tests/oracle/`), byte-comparing every `Level.Tile` and
  `Associated.Bytes` against Python opentile 0.20.0 across all 5
  sample slides.

### Architecture invariants

- Format-specific quirks live in format packages, not `internal/tiff`.
- cgo narrowly scoped to `internal/jpegturbo/`.
- Lock-free hot path for metadata; `Tile()` is concurrent-safe.
- Parity with upstream is the correctness bar.

## [0.1.0] — 2026-04-19

Initial functional milestone. Aperio SVS tiled-level passthrough.

### Added

- SVS pyramid levels (JPEG and JPEG 2000 compressions).
- TIFF parser in `internal/tiff` (classic TIFF only at this point).
- Public `Tiler` / `Level` / `AssociatedImage` interfaces.
- Three real-slide fixtures: CMU-1-Small-Region.svs, CMU-1.svs (JPEG),
  JP2K-33003-1.svs (JP2K passthrough).

[Unreleased]: https://github.com/cornish/opentile-go/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/cornish/opentile-go/releases/tag/v0.5.1
[0.5.0]: https://github.com/cornish/opentile-go/releases/tag/v0.5.0
[0.4.0]: https://github.com/cornish/opentile-go/releases/tag/v0.4.0
[0.3.0]: https://github.com/cornish/opentile-go/releases/tag/v0.3.0
[0.2.0]: https://github.com/cornish/opentile-go/releases/tag/v0.2.0
[0.1.0]: https://github.com/cornish/opentile-go/tree/feat/v0.1
