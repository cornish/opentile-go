# Changelog

All notable changes to opentile-go are recorded here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) loosely;
versioning is semantic (`MAJOR.MINOR.PATCH`).

The single source of truth for "what was deferred and why" is
[`docs/deferred.md`](docs/deferred.md). This file is the curated
front-page summary; the deferred file has the full reasoning,
upstream references, and retirement audit per milestone.

## [Unreleased]

Active limitations after v0.8: L4, L5, L14 (Permanent — carried over
from v0.6) plus L19, L20 (v0.7 work items, still deferred — fixture-
or research-driven), and L23, L24, L25 (v0.8 IFE work items deferred
to v0.9+; see `docs/deferred.md` §2). L22 (IFE METADATA block parsing)
was retired by the v0.8 metadata closeout. Open work parked in tracked
issues:

- **R4 / R9** ([#1](https://github.com/cornish/opentile-go/issues/1)) —
  SVS corrupt-edge reconstruct + JP2K decode/encode. No local SVS slide
  exhibits the corrupt-edge bug; work parked until one motivates it.
- **R6** ([#2](https://github.com/cornish/opentile-go/issues/2)) —
  3DHistech TIFF. Niche MRXS conversion target; never encountered in
  the wild. Trigger-driven park.
- **R15** ([#3](https://github.com/cornish/opentile-go/issues/3)) —
  Sakura SVSlide. Trigger-driven park.

## [0.8.0] — 2026-05-01

Iris File Extension (IFE) v1.0 support — **the first non-TIFF format
opentile-go reads**, and the first format opentile-go ships with no
Python or external-binary parity oracle. One real fixture
(`cervix_2x_jpeg.iris`, 2.16 GB, JPEG-encoded) round-trips through
`opentile.OpenFile` cleanly: 9 levels native-first, 256×256 tiles,
JPEG SOI markers on every decoded tile.

The plumbing refactor (`FormatFactory.SupportsRaw` + `OpenRaw` +
`RawUnsupported` base) ships alongside; the existing five TIFF
factories embed `RawUnsupported` for backward-compat zero-cost.

### Added

- **Iris IFE format** — `formats/ife/`. Magic-byte sniff
  (`0x49726973` LE) via `Factory.SupportsRaw` runs *before*
  `tiff.Open` in the dispatch loop. FILE_HEADER (38 B) → TILE_TABLE
  (44 B) → LAYER_EXTENTS (16-B header + 12-B entries) →
  TILE_OFFSETS (16-B header + 8-B entries with 40+24-bit
  offset/size encoding) parsing in pure Go via stdlib
  `encoding/binary`. Layer ordering inverted at parse time
  (file is coarsest-first; API exposes native-first). Tile bytes
  are self-contained — no JPEGTables splice, distinct from
  SVS/BIF abbreviated-scan pattern.
- **`FormatFactory.SupportsRaw(io.ReaderAt, int64) bool`** +
  **`FormatFactory.OpenRaw(r, size, *Config) (Tiler, error)`** —
  additive interface evolution for non-TIFF dispatch.
  `RawUnsupported` zero-impl base struct provides
  `SupportsRaw → false` / `OpenRaw → ErrUnsupportedFormat`
  defaults; the existing five TIFF factories embed it. Dispatch
  reorder in `opentile.Open` walks `SupportsRaw` first, then
  falls through to the TIFF path.
- **`opentile.FormatIFE`** constant.
- **`opentile.CompressionAVIF`** + **`opentile.CompressionIRIS`**
  enum values. `CompressionAVIF`: tile bytes are an AVIF image;
  consumer decodes via libavif. `CompressionIRIS`: Iris-proprietary
  codec; opentile-go reports but does not decode (consumer embeds
  an Iris codec or 501s).
- **`opentile.ErrSparseTile`** sentinel (wrapped in `TileError`).
  Returned when a tile position falls within the level grid but
  the format encodes "absent" via a sentinel offset (IFE's
  `NULL_TILE = 0xFFFFFFFFFF` in the 40-bit offset field).
- **`tests/parity/ife_geometry_test.go`** — per-fixture geometry
  pinning (no build tag, runs in `make test`).
- **`tests/fixtures/cervix_2x_jpeg.ife.json`** — sampled-tile SHA
  fixture. `TestSlideParity` now passes 17/17 slides
  (5 SVS + 3 NDPI + 4 Philips + 2 OME + 2 BIF + 1 IFE).
- **Synthetic-IFE-writer test harness** (`formats/ife/synthetic_test.go`
  + `formats/ife/metadata_test.go`) — hand-rolled IFE byte buffers
  cover layer inversion, sparse tiles, IRIS / AVIF encoding mappings,
  iterator order, open-time error paths, and full METADATA round-trip
  with attributes / images / ICC, without depending on the real fixture.
- **IFE METADATA block parsing** — full reader for METADATA +
  ATTRIBUTES + IMAGE_ARRAY + ICC_PROFILE (skips ANNOTATIONS for
  v0.9+). `Tiler.Metadata()` populates `Magnification` from the
  header f32; `Tiler.ICCProfile()` returns the embedded color
  profile bytes; `Tiler.Associated()` exposes IMAGE_ARRAY entries
  with normalised `Kind()` ("label" / "overview" / "thumbnail" /
  "macro" / "map" / "probability"; unknown titles surface
  lowercased). New `ife.Metadata` struct + `ife.MetadataOf(tiler)`
  accessor for IFE-specific fields: `MicronsPerPixel`,
  `MagnificationFromHeader`, `CodecMajor/Minor/Build`,
  `AttributesFormat`, `AttributesVersion`, and the free-form
  `Attributes map[string]string`. Cervix surfaces 24 attributes
  (every `aperio.*` / `tiff.*` key its source GT450 SVS carried
  before the Iris re-encode) + a 6064-byte ICC profile + a
  1920×1337 JPEG thumbnail.

### Changed

- **`opentile.Open`** dispatch order: `SupportsRaw` walked before
  `tiff.Open`. Backward-compat verified — every existing fixture
  still routes through the TIFF path because every TIFF factory's
  `SupportsRaw` (via `RawUnsupported`) returns false.
- **`FormatFactory` interface** gains two methods (additive). Format
  packages whose files are TIFF-based embed `RawUnsupported`;
  non-TIFF packages override both methods.

### Deviations from upstream

Two new deliberate divergences (see
[`docs/deferred.md` §1a](docs/deferred.md) for full reasoning):

- **Non-TIFF dispatch path** — `FormatFactory.SupportsRaw` +
  `OpenRaw` + `RawUnsupported`. Backward-compatible via embedded
  defaults. The first table-driven non-TIFF dispatch in the
  module.
- **`TILE_TABLE.x_extent` / `y_extent` ignored** for level
  dimensions on IFE. Spec doc claims these are "image
  width/height in pixels at top resolution layer," but the
  cervix fixture stores tile counts (matching `LAYER_EXTENTS.x_tiles`).
  Reader derives image dims from `LAYER_EXTENTS × 256` instead.

### Deferred (v0.9+)

- **L23** — Cross-tool parity vs `tile_server_iris` HTTP output.
  v0.8 ships sample-tile SHA fixtures + synthetic-writer + per-
  fixture geometry as the correctness bar. Cross-language byte-
  equality oracle is a future follow-up.
- **L24** — AVIF + Iris-proprietary tile decode is consumer's
  responsibility (Permanent — design choice). opentile-go reports
  `CompressionAVIF` / `CompressionIRIS` so consumers know the
  codec; linking libavif or an Iris codec would expand the cgo
  footprint past `internal/jpegturbo/` and break the byte-
  passthrough contract.
- **L25** — IFE ANNOTATIONS block parsing. v0.8 validates the
  offset is in-bounds but doesn't parse contents. Cervix has
  `annotations_offset == NULL_OFFSET`, so this is fixture-driven —
  resolved when a real annotated IFE surfaces.

### Notes

- IFE has **no Python parity oracle**. v0.7's tifffile + opentile-py
  oracles can't read IFE; openslide doesn't either. Coverage is
  layered: sample-tile SHAs lock in opentile-go's own output across
  regressions; synthetic-writer tests catch reader bugs without
  depending on a real fixture; per-fixture geometry pinning catches
  dimension regressions. The first cross-tool divergence story we
  hit will be debugged from scratch — acceptable risk for a
  bleeding-edge format.
- The plumbing refactor is the second additive interface evolution
  in two milestones (v0.7 added `Level.TileOverlap` + `TileAt` +
  `Image.Size{Z,C,T}`; v0.8 adds `FormatFactory.SupportsRaw` +
  `OpenRaw`). Both paid for themselves — TileOverlap is non-zero
  on BIF level 0; SupportsRaw is what makes IFE possible.

## [0.7.0] — 2026-04-28

Ventana BIF (Roche / iScan) support — the first opentile-go format
beyond upstream Python opentile's coverage. Two real fixtures
(`Ventana-1.bif` spec-compliant DP 200 + `OS-1.bif` legacy iScan
Coreo) round-trip through `opentile.OpenFile` cleanly. Correctness
is anchored on **tifffile byte-equality** for the spec-compliant
path + **committed sample-tile SHA256 hashes** for both fixtures via
`TestSlideParity`.

### Added

- **Ventana BIF format** — `formats/bif/`. BigTIFF detection via
  `<iScan` substring match in any IFD's XMP. Generation
  classification by `strings.HasPrefix(scannerModel, "VENTANA DP")`
  (DP 200, DP 600, future DP scanners → spec-compliant path; else
  → legacy-iScan path). IFD classification by `ImageDescription`
  content. Pyramid levels sorted by parsed `level=N`. Per-tile
  serpentine remap (image-space (col, row) → physical-stage
  TileOffsets index). Empty-tile path returns a cached blank JPEG
  filled with `<iScan>/@ScanWhitePoint` luminance (default 255 when
  the attribute is absent). Shared JPEGTables (tag 347) spliced via
  `internal/jpeg.InsertTables` (no Adobe APP14 — BIF is YCbCr).
- **`internal/bifxml/`** — stdlib `encoding/xml` walkers for
  `<iScan>` and `<EncodeInfo>` XMP blocks. Lenient parsing; ordinal
  `<AOI<N>>` iteration; out-of-range `ScanWhitePoint` clamped;
  `<EncodeInfo>` Ver < 2 rejected per spec.
- **`Level.TileOverlap() image.Point`** interface method (additive).
  Returns the per-tile-step pixel overlap; non-zero only on BIF
  level 0. Both real fixtures carry non-zero overlap on level 0
  (Ventana-1=(2,0); OS-1=(18,26)) — contrary to the original v0.7
  design spec §10's "fixture-untested" claim. Other formats return
  `image.Point{}`.
- **`bif.MetadataOf(opentile.Tiler) (*Metadata, bool)`** — exposes
  Generation, ScanRes, ScanWhitePoint+Present, ZLayers,
  ImageDescription, AOIs, AOIOrigins, EncodeInfoVer. Walks
  `UnwrapTiler` chains.
- **`opentile.FormatBIF`** constant.
- **`internal/tiff.TagXMP`** (700) + `Page.XMP()`,
  **`TagImageDepth`** (32997) + `Page.ImageDepth()`,
  **`TagDateTime`** (306).
- **AssociatedImage `kind="probability"`** — new kind value joining
  the existing taxonomy. Spec-compliant DP 200 fixtures expose IFD 1
  as the LZW-compressed tissue probability map.
- **`formats/bif/blanktile.go`** — cached JPEG blank-tile generator.
- **Three parity oracles**: `tests/parity/bif_geometry_test.go` (no
  build tag, runs in `make test`); `TestTifffileParityBIF`
  (Ventana-1, byte-equality); `TestOpenslideBIFParity`
  (infrastructure-only in v0.7, `t.Skip`'d for v0.8 follow-up).
- Sampled-tile fixtures for both BIF fixtures. `TestSlideParity` now
  passes 16/16 slides (5 SVS + 3 NDPI + 4 Philips + 2 OME + 2 BIF).
- **Multi-dimensional addressing** —
  `Level.TileAt(TileCoord{X, Y, Z, C, T})` plus
  `Image.SizeZ/SizeC/SizeT/ChannelName/ZPlaneFocus`. Additive;
  2D formats inherit `SingleImage` defaults (`SizeZ/SizeC/SizeT == 1`)
  and `Tile(x, y) == TileAt(TileCoord{X: x, Y: y})` byte-identically.
  New `ErrDimensionUnavailable` sentinel discriminates "axis absent"
  (`SizeZ == 1` + `Z != 0`) from "axis index past size"
  (`ErrTileOutOfBounds`).
- **BIF multi-Z reading** via the `IMAGE_DEPTH` (32997) tag. BIF
  level 0 with `imageDepth > 1` exposes nominal + near + far focus
  planes through `TileAt(TileCoord{Z: z})`; `Image.ZPlaneFocus(z)`
  returns the per-plane Z-spacing offset (Z=0 nominal, Z=1..nNear
  near = negative offsets, Z=nNear+1..N-1 far = positive offsets)
  parsed from `<iScan>/@Z-spacing`. Synthetic fixture coverage in
  `formats/bif/multiz_test.go`; no real volumetric BIF in
  `sample_files/`.
- **OME-TIFF honest dimension reporting** — `Image.SizeZ/SizeC/SizeT`
  reflect `<Pixels SizeZ/SizeT>` and `<Channel>` element count
  (intentionally NOT `<Pixels SizeC>`, which describes per-pixel
  RGB sample count rather than separately-stored channels). Both
  Leica fixtures still report `SizeZ/SizeC/SizeT == 1`.
  `Level.TileAt(TileCoord{Z != 0})` returns
  `ErrDimensionUnavailable` until the per-IFD reader lands as a
  separate format-package milestone (sketched in
  `docs/formats/ome.md`).

### Changed

- **`Level` interface** gains `TileOverlap() image.Point` and
  `TileAt(TileCoord) ([]byte, error)` — additive evolution;
  existing concrete level types grow zero-returning /
  delegate-to-`Tile` impls. No caller change required for non-BIF
  formats.
- **`Image` interface** gains `SizeZ/SizeC/SizeT/ChannelName/
  ZPlaneFocus` — additive evolution; `SingleImage` provides
  defaults so 2D formats compile without changes.

### Deviations from upstream Python opentile

One new deliberate divergence (see
[`docs/deferred.md` §1a](docs/deferred.md) for full reasoning):

- **Multi-dimensional WSI API addition** — `TileCoord` +
  `Level.TileAt` + `Image.SizeZ/SizeC/SizeT/ChannelName/ZPlaneFocus`.
  Additive across all formats. Modern WSI consumers (fluorescence,
  focal-plane viewers, time series) need explicit multi-dim
  addressing; upstream Python opentile is 2D-only.

### Deferred (v0.8+)

- **L19** — openslide pixel-equivalence on BIF
  (infrastructure-only in v0.7; coordinate-system gap between
  opentile-go's padded TIFF grid and openslide's AOI-hull view).
- **L20** — DP 600 (and other future "VENTANA DP *") behavioural
  variance — unverified without a fixture.

### Retired (mid-v0.7)

- **L21** — Volumetric Z-stacks. The v0.7 multi-dim closeout
  introduced cross-format multi-dim addressing; BIF now reads
  the entire `IMAGE_DEPTH` Z-stack natively (Z=0 nominal + nNear
  near planes + nFar far planes). OME surfaces honest dimensions
  via `Image.SizeZ/SizeC/SizeT` and defers `TileAt(z != 0)` to a
  future format-package milestone — that work is not L21; it's
  a fresh OME-package work item gated on a real multi-Z OME
  fixture surfacing.

### Notes

- The original v0.7 design spec (§7) framed openslide
  pixel-equivalence as the primary correctness oracle.
  Mid-implementation we found openslide rejects spec-compliant DP
  200 BIFs (`Direction="LEFT"`) and uses an AOI-hull coordinate
  system that doesn't match opentile-go's padded TIFF view.
  Anecdotal community note: openslide is also believed to misread
  modern BIF generally. The v0.7 correctness bar is therefore
  tifffile + committed sample-tile SHAs, not openslide.
- v0.7 surfaced two correctness bugs caught only by writing the
  integration test (T19): `loadEncodeInfo` was silently swallowing
  the Ver<2 rejection; `bif.MetadataOf` didn't unwrap the file-
  closer Tiler. Both fixed in `49849a4`.

## [0.6.0] — 2026-04-27

OME-TIFF support — the fourth format opentile-go handles, closing the
upstream Python opentile 0.20.0 format set. Output is byte-identical to
**Python opentile 0.20.0 + tifffile** across every sampled tile and
every associated image we expose, on both Leica fixtures.

### Added

- **OME-TIFF format** — `formats/ome/`. Tiled levels with SubIFD-based
  pyramid traversal; OneFrame (non-tiled) levels via the new shared
  `internal/oneframe/` package; macro / label / thumbnail associated
  images; OME-XML metadata via stdlib `encoding/xml`. Two fixtures
  in the parity slate (`Leica-1.ome.tiff`, `Leica-2.ome.tiff`).
- **`Image` interface + `Tiler.Images() []Image`** (additive public API).
  Multi-image OME-TIFF files (Leica-2 carries 4 main pyramids) expose
  every pyramid via `Images()`. Single-image formats (SVS, NDPI,
  Philips) return a one-element slice via the new `opentile.SingleImage`
  helper. Existing `Tiler.Levels()` / `Level(i)` keep working as
  documented shortcuts to `Images()[0]`.
- **`opentile.FormatOME`** constant.
- **`internal/tiff.TagSubIFDs`** (TIFF tag 330) +
  **`Page.SubIFDOffsets()`** accessor.
- **`internal/tiff.File.PageAtOffset(off)`** for SubIFD traversal.
- **`internal/oneframe/`** package — factored from
  `formats/ndpi/oneframe.go` so OME (and later v0.7 BIF) reuse the
  same machinery. New `Options.FirstStripOnly` flag for OME's
  multi-strip planar pages.
- **`internal/jpegturbo` warning tolerance** — distinguishes
  `TJERR_WARNING` from fatal via `tjGetErrorCode`; treats warnings as
  success when `*dst` is populated. Required for OME OneFrame's
  truncated scan data; NDPI parity preserved.
- **`tests/oracle/tifffile_runner.py`** + **`tests/oracle/tifffile_session.go`** —
  new tifffile-based parity oracle covering every Image's tiled levels,
  including the 3 Leica-2 main pyramids opentile-py drops via its
  last-wins loop.
- **Per-format docs** under `docs/formats/` — one .md per format
  (svs, ndpi, philips, ome) with capability matrix, deviations, fix
  history, and upstream references.
- **Canonical `Deviations` section** in `docs/deferred.md` §1a.

### Changed

- **README rewritten** for public consumption. New format-support
  summary table; comprehensive API guide including the multi-image
  `Tiler.Images()` flow; "Deviations" subsection. Drops "Pure-Go"
  claim — opentile-go has one cgo dependency. Builds without cgo
  via `-tags nocgo` (SVS-only / NDPI-striped consumers unaffected).
- **Fixture schema** gained `Images []ImageFixture` for multi-image
  formats. Single-image fixtures unchanged.
- `internal/tiff.Page.scalarU32` falls through to `Values64` for
  BigTIFF LONG8/IFD8 scalar values — discovered while wiring SubIFD
  reads on the Leica fixtures, where `ImageWidth` / `ImageLength`
  were silently failing.

### Deviations from upstream Python opentile

Three new deliberate divergences (see
[`docs/deferred.md` §1a](docs/deferred.md) for full reasoning):

- **Multi-image OME pyramid exposure**: upstream's last-wins loop
  silently drops 3 of 4 main pyramids in `Leica-2.ome.tiff`; we
  expose all of them via `Tiler.Images()`. Use `Tiler.Levels()` for
  first-image-only behaviour.
- **PlanarConfiguration=2 plane-0-only indexing**: matches Python's
  silent flat-indexing into per-channel-tripled offset arrays.
- **First-strip-only on multi-strip OneFrame**: matches Python's
  `_read_frame(0)` behaviour on `rowsperstrip × samplesperpixel`
  planar pages.

### Retired

- **R7** (OME TIFF) — landed end-to-end. `docs/deferred.md §8` has
  the v0.6 retirement audit + the five JIT-gate outcomes (T1
  detection, T2 SubIFD parsing, T3 OneFrame factor decision, T4
  OME-XML schema, T5 tifffile splice-replication harness).

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

[Unreleased]: https://github.com/cornish/opentile-go/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/cornish/opentile-go/releases/tag/v0.8.0
[0.7.0]: https://github.com/cornish/opentile-go/releases/tag/v0.7.0
[0.6.0]: https://github.com/cornish/opentile-go/releases/tag/v0.6.0
[0.5.1]: https://github.com/cornish/opentile-go/releases/tag/v0.5.1
[0.5.0]: https://github.com/cornish/opentile-go/releases/tag/v0.5.0
[0.4.0]: https://github.com/cornish/opentile-go/releases/tag/v0.4.0
[0.3.0]: https://github.com/cornish/opentile-go/releases/tag/v0.3.0
[0.2.0]: https://github.com/cornish/opentile-go/releases/tag/v0.2.0
[0.1.0]: https://github.com/cornish/opentile-go/tree/feat/v0.1
