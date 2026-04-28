# BIF format research — 2026-04-27

Raw evidence supporting the v0.7 design doc
(`docs/superpowers/specs/2026-04-27-opentile-go-v07-design.md`).
This file is the receipts; the design doc is the synthesis.

Sources:
1. `sample_files/ventana-bif/Roche-Digital-Pathology-BIF-Whitepaper.pdf`
   (Roche, v1.0, 2020-11-19, MC--06058 1120, 17 pages).
2. Fixture probe via `tifffile` against
   `sample_files/ventana-bif/Ventana-1.bif` and
   `sample_files/ventana-bif/OS-1.bif`.
3. `openslide-show-properties` against the same two fixtures.
4. openslide's `src/openslide-vendor-ventana.c` from the upstream
   `openslide/openslide` repository (LGPL 2.1 — read-for-understanding
   only, never copied verbatim into opentile-go).

---

## 1. Whitepaper digest

**Container.** BigTIFF, little-endian (magic `49 49 2B 00`). Up to
~18 EB; 64-bit offsets. Confirmed against both local fixtures.

**Mandatory scanner gate** *(per spec)*. IFD 0 → XMP → `<iScan>` →
`ScannerModel` attribute must equal `"VENTANA DP 200"`; spec says
"Stop processing the BIF-file if the string does not match model
name." iScan Coreo and iScan HT are explicitly **out of scope** of
this whitepaper.

**IFD layout** *(per spec, DP 200 only)*:

| IFD | Role | Compression | Layout | Other tags |
|-----|------|-------------|--------|------------|
| 0 | Overview / label image | JPEG | striped, sRGB | XMP, ImageDescription `'Label_Image'` |
| 1 | Tissue probability map | LZW | striped, 8-bit gray | XMP, ImageDescription `'Probability_Image'` |
| 2 | High-res scan (level 0) | JPEG | tiled | XMP `<EncodeInfo>`, ICC profile (tag 34675, only here), YCbCrSubsampling, ReferenceBlackWhite |
| 3+ | Dyadic pyramid (levels 1..N) | JPEG | tiled | no XMP, no ICC |

**ImageDescription on IFD 2+**: 3 SPACE-separated KEY=VALUE tokens —
`level=0 mag=40 quality=95`. Use these to identify pyramid order and
read per-level magnification (the spec says "Do not compute the
magnification using other data").

**Tile size**. Typically square ~1024×1024, but XMP `XIMAGESIZE` /
`YIMAGESIZE` give the actual size; non-square is allowed
(OS-1 fixture uses 1024×1360).

**Tile overlap (IFD 2 only)**. Stored in XMP `EncodeInfo →
SlideStitchInfo → ImageInfo → TileJointInfo` nodes. Per pair:
`OverlapX`, `OverlapY` in pixels. **DP 200 only ever produces
horizontal overlap (`OverlapY=0`)** — spec is explicit. Pyramid IFDs
3+ have no overlap.

**Empty tiles** (multi-AOI gaps). `TileOffsets[i] = 0` and
`TileByteCounts[i] = 0` indicates an unscanned tile; consumer should
fill with the `ScanWhitePoint` value from the IFD 0 XMP. Same shape as
the Philips sparse-tile path (R5).

**JPEGTables (tag 347)**. *Optional*. Older BIFs embed full JPEG
headers in each tile; newer BIFs share via tag 347. Both are valid
within the spec; readers must handle both.

**Multi-AOI**. The high-res image is the convex hull of all AOIs;
gaps are filled with empty tiles. AOI origin pixel coords are stored
in `EncodeInfo → AoiOrigin → AOI<N>` (`OriginX`, `OriginY`); always
multiples of tile size.

**Serpentine ordering**. `TILE_OFFSETS` is in physical-stage
serpentine order, not row-major image order. Mapping is in the XMP
`<Frame>` nodes inside `EncodeInfo → SlideStitchInfo → ImageInfo →
FrameInfo → Frame` (one per tile), each with `XY="C,R"` for
column/row in image-space. Reader must permute serpentine →
row-major to expose `Tile(col, row)`.

**Volumetric Z-stacks** via private TIFF tag IMAGE_DEPTH = 32997
(0x80E5). First M×N tile entries = nominal focus plane, followed by
Z-1 additional planes. Whitepaper says non-Z-aware readers can simply
read the first M×N tiles. Spec-internal inconsistency: the body text
on page 5 calls it `0x80BE`, but page 6 and Appendix A consistently
use `0x80E5 = 32997`. Treat 32997 as authoritative (matches
SGI-registered private tag).

**EncodeInfo Ver**. Must be ≥ 2; spec says stop processing if not.

**Color**. Pixel data is in device-dependent color space; correct
display requires applying ICC profile (tag 34675, only in IFD 2).
Surface as `Tiler.ICCProfile()`. Out of scope to apply in a tile
reader.

---

## 2. Fixture probe

```
========== Ventana-1.bif ========== (227 MB)
BigTIFF: True, byteorder: <, #pages: 10
IFD0:    1251x3685   comp=NONE(1)   strips      desc='Label_Image'
IFD1:    1251x3685   comp=LZW(5)    strips      desc='Probability_Image'
IFD2:   24576x21504  comp=JPEG(7)   1024x1024   desc='level=0 mag=40 quality=95'  N=504
IFD3:   12288x10752  comp=JPEG(7)   1024x1024   desc='level=1 mag=20 quality=95'  N=132
IFD4:    6144x5376   comp=JPEG(7)   1024x1024   desc='level=2 mag=10 quality=95'   N=36
IFD5:    3072x2688   comp=JPEG(7)   1024x1024   desc='level=3 mag=5 quality=95'    N=9
IFD6:    1536x1344   comp=JPEG(7)   1024x1024   desc='level=4 mag=2.5 quality=95'  N=4
IFD7:     768x672    comp=JPEG(7)   1024x1024   desc='level=5 mag=1.25 quality=95' N=1
IFD8:     384x336    comp=JPEG(7)   1024x1024   desc='level=6 mag=0.625 quality=95' N=1
IFD9:     192x168    comp=JPEG(7)   1024x1024   desc='level=7 mag=0.3125 quality=95' N=1

IFD0 XMP highlights:
  ScannerModel = "VENTANA DP 200"      ← spec-mandated value present
  BuildVersion = "1.1.0.15854"
  BuildDate    = "11/27/2019"
  ScanWhitePoint = 235
  AOI0 Left=297 Top=2323 Right=574 Bottom=2069 (single AOI)

IFD2 XMP highlights:
  EncodeInfo Ver = "2"                 ← spec-mandated minimum met
  AoiInfo XIMAGESIZE=1024 YIMAGESIZE=1024 NumRows=21 NumCols=23 (= 483 tiles + padding)
  TileJointInfo Direction="LEFT"       ← openslide explicitly rejects this value
  TileJointInfo OverlapX=0 OverlapY=0  ← all zero on this slide → no actual overlap

JPEGTables tag (347): NOT PRESENT  ← per-tile embedded JPEG headers
ICCProfile tag (34675): present on IFD2

========== OS-1.bif ========== (3.6 GB)
BigTIFF: True, byteorder: <, #pages: 12
IFD0:    1008x3008   comp=JPEG(7)   1008x3008  (single tile)  desc='Label Image'
IFD1:    1024x912    comp=JPEG(7)   1024x912   (single tile)  desc='Thumbnail'
IFD2:  118784x102000 comp=JPEG(7)   1024x1360                 desc='level=0 mag=40 quality=90'  N=8700
IFD3:   59392x51000  comp=JPEG(7)   1024x1360                 desc='level=1 mag=20 quality=90'  N=2204
... (10 pyramid levels total, each 2x downsampled)
IFD11:    232x200    comp=JPEG(7)   1024x1360                 desc='level=9 mag=0.078125 quality=90'

IFD0 XMP highlights:
  ScannerModel = (NOT PRESENT)         ← would FAIL the spec's gate
  BuildVersion = "3.3.1.1"
  BuildDate    = "December, 13 2011"
  Magnification = 40
  ScanRes = 0.232500                   ← matches openslide's mpp 0.2325

IFD2 XMP highlights:
  EncodeInfo Ver = "2"
  AoiInfo XIMAGESIZE=1024 YIMAGESIZE=1360 NumRows=75 NumCols=116
  TileJointInfo Direction="RIGHT"      ← openslide accepts this
  TileJointInfo FlagJoined='0' OverlapX='0' OverlapY='0'  ← unjoined, zero overlap

JPEGTables tag (347): PRESENT          ← shared JPEG tables, newer encoding
ICCProfile tag (34675): present on IFD2
```

The two fixtures are different format generations and split openslide
compatibility opposite ways:

|                              | Ventana-1.bif  | OS-1.bif        |
|------------------------------|----------------|-----------------|
| ScannerModel                 | VENTANA DP 200 | (missing)       |
| Spec-compliant?              | yes            | no (pre-DP200)  |
| openslide reads it?          | **no** (Direction="LEFT") | **yes** |
| JPEGTables tag               | absent         | present         |
| IFD 1 role                   | probability    | thumbnail       |
| IFD 0 compression            | uncompressed   | JPEG (single tile) |
| Direction values seen        | LEFT           | RIGHT           |
| Actual overlap pixels        | 0              | 0               |

Notable: spec-compliant Ventana-1 IFD 0 is *uncompressed* (comp=1),
not JPEG as the whitepaper text implies. Reader must not assume
IFD 0 compression.

---

## 3. openslide ventana.c reader analysis

(Read-for-understanding only; LGPL 2.1; do **not** copy code verbatim.)

**Detection.** Scans for the literal string `"iScan"` inside the
XMLPacket of any TIFF page. No `ScannerModel` validation; openslide
accepts any iScan-tagged BigTIFF regardless of scanner generation.
This is more permissive than the BIF whitepaper's strict DP 200 gate.

**Direction quirk.** Hard-codes acceptance of `RIGHT` and `UP` only;
`LEFT` and `DOWN` raise `"Bad direction attribute"` and abort. This
is why openslide cannot read Ventana-1.bif. The whitepaper lists all
four as valid for `<TileJointInfo>`.

**Tile overlap policy.** Exposes overlap as metadata, never crops
pixel data. Per-pair `OverlapX/OverlapY` are negated and stored as
`tile_advance_x / tile_advance_y` — a weighted-average displacement
that the renderer applies when positioning tiles. Raw tile bytes go
through unchanged.

**Serpentine remap.** Boustrophedon flip on odd rows
(`col = tiles_across - col - 1`) plus full row inversion
(`row = tiles_down - row - 1`) since stage rows count up from bottom
but image rows count down from top.

**Empty/unscanned tiles.** Not specially handled. Spec mandates
filling with `ScanWhitePoint`; openslide just delegates to the TIFF
layer. (We should do the spec-correct thing.)

**Multi-AOI.** Each AOI becomes a separate "region" exposed as
`openslide.region[i].(x|y|width|height)` properties; the merged
single-pyramid model is preserved with empty tiles between AOIs.

**Associated images.** Identified by ImageDescription value:
`"Label Image"` or `"Label_Image"` → macro; `"Thumbnail"` →
thumbnail. No probability-image surfacing (not exposed).

**Properties from XMP.** Most `<iScan>` attributes are mirrored 1:1
as `ventana.<Attribute>` properties (Magnification, ScanRes,
UnitNumber, BuildVersion, …). MPP (`openslide.mpp-x/y`) is computed
from `ScanRes`.

---

## 4. openslide CLI dump comparison

```
$ openslide-show-properties Ventana-1.bif
openslide-show-properties: ... Bad direction attribute "LEFT"
[exits with error]

$ openslide-show-properties OS-1.bif
openslide.level-count: 10
openslide.level[0].width:  105813
openslide.level[0].height:  93951
openslide.mpp-x: 0.2325
openslide.objective-power: 40
openslide.vendor: ventana
openslide.region[0].(x|y|width|height): 0 0 105813 93951  (single AOI)
openslide.associated.macro.(width|height): 1008 3008
openslide.associated.thumbnail.(width|height): 1024 912
[+ many ventana.* properties from XMP]
```

(Note: openslide reports level 0 as 105813×93951 but the IFD 2 raw
dimensions are 118784×102000 — openslide is cropping to the
hull-of-AOIs rectangle, not the padded tile rectangle. Worth checking
how the spec defines the "official" image extent before deciding what
opentile-go's Level dimensions should be.)
