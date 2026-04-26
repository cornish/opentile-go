# Deferred Issues

Running log of items that are intentionally out of scope for the current milestone or were raised during code review and parked for later. Once this branch lands on a hosting service, triage everything below into real tracked issues and delete the entries as they are filed.

Format per item:
- **ID** — short descriptor
- **Source** — commit or review where the item originated
- **Severity** — Scope / Limitation / Suggestion
- **Disposition** — target milestone or backlog note

---

## 1. Roadmap items (scope-deferred by design)

Not bugs; intentional deferral in the design phase. ✅ = retired (landed in a prior or current milestone).

**v0.4 scope:** finish what we already support — close every real bug
and parity gap on SVS and NDPI before adding any new format.

**v0.5+ scope:** new format support (Philips, 3DHistech, OME).

| ID | Feature | Target | Status |
|----|---------|--------|--------|
| R1 | NDPI format support (Hamamatsu) | v0.2 | ✅ landed (Batches 2-7, parity verified) |
| R2 | `internal/jpeg` marker package | v0.2 | ✅ landed (Batch 2) |
| R3 | SVS associated images — label, overview, thumbnail | v0.2 (promoted from v0.3) | ✅ landed (Task 21, `9cd27cb`) |
| R4 | Aperio SVS corrupt-edge reconstruct fix (currently returns `ErrCorruptTile`) | v0.4 | deferred (was v1.0; promoted as part of SVS-completeness focus) |
| R5 | Philips TIFF (sparse-tile filler) | v0.5 | deferred (was v0.4; demoted in favour of v0.4 SVS/NDPI completeness) |
| R6 | 3DHistech TIFF | v0.5 | deferred |
| R7 | OME TIFF | v0.5 | deferred |
| R8 | BigTIFF support | v0.2 | ✅ landed (Batch 1) |
| R9 | JPEG 2000 decode/encode (currently passes through native tiles; decode matters for associated-image re-encoding and corrupt-tile reconstruct) | v0.4 | deferred (was v0.3+; needed by R4) |
| R10 | Remote I/O backends (S3, HTTP range, fsspec equivalents) | out-of-scope; consumers supply `io.ReaderAt` | permanent |
| R11 | Python parity oracle under `//go:build parity` | v0.2 | ✅ landed (Task 25-26, Batch 7) |
| R12 | CLI wrapper | out-of-scope for v1 | permanent |
| R13 | NDPI Map (`mag == -2.0`) pages exposed as associated images | v0.4 | deferred (was v0.3+; same milestone as L6) |

---

## 2. Active limitations (open after v0.3)

Real behaviour gaps that have not been closed. Each entry's **Severity**
field declares whether it's a permanent design choice (the library
cannot reasonably address it without breaking compatibility) or a
v0.4 work-item (a real fix is queued for the existing-format
completeness milestone). Items closed during v0.3 are listed in §5.

### L4 — MPP may be absent from some slides' ImageDescription
- **Source:** diagnostic run during Task 20
- **Severity:** Permanent — input-data-dependent. The library cannot fabricate MPP values that the slide doesn't carry; callers must handle the zero case.
- **Detail:** CMU-1 embeds `MPP = 0.4990` in its ImageDescription, but not every Aperio slide does. When absent, `svs.Metadata.MPP` is zero and every Level's `MPP()` returns `SizeMm{0, 0}`. No bug, but a caller who expects non-zero MPP should check `SizeMm.IsZero()` rather than assuming a value.

### L5 — `internal/tiff` peeks at NDPI-specific tag to select IFD layout
- **Source:** Architecture audit (post-Batch 4)
- **Severity:** Permanent — design decision. NDPI files share classic TIFF magic 42 with no header-level distinguisher, so something has to peek to dispatch the NDPI-layout walker. The peek is encapsulated in `sniffNDPI` (see N-9 godoc on that function).
- **Detail:** `internal/tiff/file.go` reads tag 65420 (Hamamatsu NDPI FileFormat) in the first IFD to decide whether to dispatch the NDPI-layout IFD walker. A cleaner split would have `tiff.Open` expose a generic "layout" hint and let format packages drive selection — but that requires callers to know NDPI exists, which inverts the dependency. Revisit only if adding Philips/OME forces a more general dialect-detection scheme.

### L6 — NDPI Map pages (`mag == -2.0`) are silently dropped  *(resolved in v0.4)*
- **Source:** NDPI classifier port (post-Batch 4)
- **Severity:** Fixed on `feat/v0.4` (Tasks 6-8) together with roadmap item R13. NDPI Map pages now surface as `AssociatedImage` entries with `Kind() == "map"`. OS-2.ndpi page 11 (580x198, uncompressed grayscale) and Hamamatsu-1.ndpi page 7 (600x205) are both exposed; CMU-1.ndpi unchanged (no Map page).
- **Original detail:** `classifyPage` returned `pageMap` for Magnification tag value -2.0, and `Factory.Open` silently dropped that kind. tifffile's `_series_ndpi` (`tifffile.py:5049-5072`) had always classified these as `series.name == 'Map'`, but upstream Python opentile chose not to surface them — `NdpiTiler` returns False from `_is_label_series` and `_is_thumbnail_series`, leaving Map pages with no predicate and no `tiler.maps` property to reach them.

  v0.4 fix is a deliberate Go-side extension: `formats/ndpi/mappage.go` adds a `mapPage` struct that mirrors `overviewImage` but uses the page's actual TIFF Compression tag (Map pages are uncompressed 8-bit grayscale on every Hamamatsu fixture we have, NOT JPEG like the other associated images). Downstream consumers decoding a Map page need to expect a single-channel image. Parallels the existing v0.2 NDPI synthesised label (L14).

### L12 — NDPI edge-tile OOB fill (control-flow bug)  *(resolved in v0.4)*
- **Source:** NDPI v0.2 architectural rewrite (feat/v0.2); root cause re-diagnosed in v0.4 Task 3 (2026-04-26).
- **Severity:** Fixed on `feat/v0.4` (Task 9). `formats/ndpi/striped.go::Tile` now dispatches geometry-first against image-size — matching Python's `turbojpeg.py:839-863` `__need_fill_background` gate exactly. CMU-1 / OS-2 / Hamamatsu-1 NDPI fixtures regenerated; parity oracle is byte-equal to Python opentile on every NDPI tile (the L12 `t.Logf` carve-out in `tests/oracle/parity_test.go` was removed in the same commit).

- **Detail:** Python opentile (`turbojpeg.py:839-863`'s `__need_fill_background`) decides geometry-first: route through CUSTOMFILTER iff `crop_region.x + crop_region.w > image_size[0]` OR `crop_region.y + crop_region.h > image_size[1]`, AND `background_luminance != 0.5`. No try-Crop-first pattern.

  Our `striped.go::Tile` tries plain `Crop` first, falls through to `CropWithBackgroundLuminanceOpts` only when `Crop` errors. For OS-2 / Hamamatsu-1 edge tiles where `Crop` succeeds despite the tile extending past the image (libjpeg-turbo accepts the MCU-aligned geometry), we silently return `Crop`'s default OOB fill — DC=0 → mid-gray (RGB 128,128,128). Python plants DC=170 → white (RGB 255,255,255).

  Verified on OS-2.ndpi L5 tile (3,0): in-image pixels (cols 0-895) match Python byte-for-byte; OOB strip (cols 896-1023) is mid-gray on Go, white on Python. Both sides individually byte-deterministic; the divergence is purely between languages and exclusively in the OOB region.

  The v0.3 T30 task (`acc2282`) attempted this fix but reverted because the v0.3 fixtures already encoded the buggy mid-gray output — the "regression" was actually the correct behaviour returning. v0.4 Task 9 landed the fix and regenerated CMU-1 / OS-2 / Hamamatsu-1 NDPI fixtures together; parity oracle is byte-equal to Python opentile on every NDPI tile.

### L14 — NDPI label synthesis is Go-specific; Python opentile does not expose NDPI labels
- **Source:** Batch 7 parity oracle extension
- **Severity:** Permanent — deliberate Go-side extension. Removable via `WithNDPISynthesizedLabel(false)` (the v0.3 N-5 fix); see `formats/ndpi/synthlabel_test.go` for the opt-out path.
- **Detail:** Python opentile 0.20.0's `NdpiTiler.labels` returns an empty
  list — the upstream library does not surface a "label" AssociatedImage
  for NDPI slides. Our Go implementation synthesizes one by cropping the
  left 30% of the overview page (`formats/ndpi.newLabelImage`,
  `formats/ndpi/associated.go`), matching a convention common in Aperio
  SVS but with no upstream NDPI precedent.

  The Batch 7 parity test accommodates this by treating a zero-length
  Python oracle output as "skip"; downstream consumers inspecting
  `Tiler.Associated()` on an NDPI slide will see a `Kind() == "label"`
  entry our Go side provides and Python's does not. Not a bug, but a
  deliberate Go-side extension that callers switching between the two
  languages should know about.

  *How to fix if we decide it's unwanted:* drop the `newLabelImage` call
  in `formats/ndpi/ndpi.go` Open() so NDPI associated is overview-only.
  Or keep it and add a README note calling it out.

### L17 — NDPI label cropH rounded to MCU multiple, not full image height  *(resolved in v0.4)*

- **Source:** L7 fix in v0.3 (Task 10) surfaced the divergence.
- **Severity:** Fixed on `feat/v0.4` (Task 5). `formats/ndpi/associated.go::newLabelImage` now passes the FULL image height to `jpegturbo.Crop` (not the MCU-floored height) — matching Python's `_crop_parameters[3] = page.shape[0]` at `opentile/formats/ndpi/ndpi_image.py:144`. libjpeg-turbo's TJXOPT_PERFECT accepts the partial last MCU row when the crop ends exactly at the image edge; PyTurboJPEG's `__need_fill_background` gate (turbojpeg.py:839-863) returns False for label crops because `crop_y + crop_h == image_h` (not `>`), so Python takes the plain-crop path, not the CUSTOMFILTER path.
- **Original detail:** Pre-v0.4 `formats/ndpi.newLabelImage` rounded the height to `(overview.size.H / mcuH) * mcuH`, dropping the last partial-MCU row. Visible on OS-2.ndpi (344x392 Go vs 344x396 Python) and Hamamatsu-1.ndpi (640x728 Go vs 640x732 Python). CMU-1.ndpi (352x408) was unaffected because its image height is divisible by mcuH.

  The pre-v0.4 deferred entry steered the fix through `CropWithBackground` "with luminance=1.0 and chroma DC=0 (matches Python)." That advice was wrong — based on an incomplete reading of upstream. Python doesn't route label crops through CUSTOMFILTER; it just passes the un-rounded image height to plain `tjTransform`, which handles the partial-MCU edge case natively. The fix turned out to be a one-line change to `cropH` plus dropping the now-unused `mcuH` argument.

  **Status (2026-04-26):** fixed. OS-2.ndpi and Hamamatsu-1.ndpi NDPI fixtures regenerated. Lesson re-learned: confirm upstream's actual code path before designing the fix — the universal Step 0 in v0.4 plan tasks exists for cases like this.

---

## 3. Reviewer suggestions accepted but not applied

Non-blocking items raised during per-task spec / quality review. The v0.3
batch closed every suggestion in this section; see §6 for the retired list.
Future review cycles re-populate this section.

---

## 4. v0.2 session learnings (process notes)

These are not deferred items — they're observations about how v0.2 development went, kept here so future sessions have context.

- **The "don't guess — read upstream" rule was established mid-session** (commit `993289e`). Codified in CLAUDE.md and in the cross-session memory system (`feedback_no_guessing.md`). Prompted by three separate "guess-and-regret" debugging cycles in v0.2 (NDPI IFD layout, NDPI metadata tag numbers, StripOffsets tag number) and reinforced by two more in Batch 6/7 (NDPI striped vs. oneframe gate on tag 322 — L13; APP14 bytes miscopied — now canonical). Future format work should read upstream first and commit behavior from there.

- **Architecture audit at end of Batch 4** — three minor cleanups landed (NDPI tiler struct consolidated into `tiler.go`, SVS `image.go` renamed to `tiled.go`, stale debug artifacts removed). Full decision trail in commit `cae47cd`.

- **"Does the slide open?" is not a sufficient NDPI smoke test (L13).** The original `formats/ndpi/striped.go` gated on TIFF tag 322 (TileWidth), which NDPI pages never carry. Every pyramid level fell through to `oneFrameImage`'s per-tile whole-level tjTransform — 3000× slower than Python opentile for CMU-1.ndpi L0, but every unit and integration test still passed because they only verified `Open()` + tile counts, never per-tile throughput. Caught during Batch 6 when fixture generation hit a 30-minute timeout on a 188 MB slide. `formats/ndpi/bench_test.go` is now a regression gate that exercises per-tile time under `NDPI_BENCH_SLIDE`.

- **Byte parity is a stronger correctness bar than it first appears (Batch 7).** The parity oracle was originally scoped as a best-effort opt-in check; extending it to `Associated.Bytes()` surfaced two latent divergences in `internal/jpeg/ConcatenateScans` (APP14 byte values and segment ordering) that no other test caught — both produced valid JPEGs, both decoded to correct pixels, both self-consistent inside our own fixtures. Only byte-level comparison against Python opentile surfaced them. Lesson: "decodes to the right pixels" is not synonymous with "correct output" when downstream consumers (wsidicomizer, DICOM pipelines) may hash the output.

- **Parity with an unconfigured upstream has hidden dimensions (Batch 7).** Python opentile 0.20.0's `OpenTile.open(slide, tile_size)` expects `int`, not `(w, h)`; passing a tuple silently worked for SVS and raised `TypeError` from NDPI's adjust_tile_size. Two separate parity agents hit this. The v0.2 runner pins `tile_size: int` and documents the quirk.

- **tjTransform's CUSTOMFILTER callback is not bitwise-deterministic across Go/Python (L12).** Same flags (`TJXOPT_PERFECT | TJXOPT_CROP`), same callback output, same source JPEG → subtly different entropy streams for NDPI edge tiles. Verified our DC coefficient math matches Python's (`round((lum * 2047 - 1024) / dc_quant) = 170` for luminance=1.0 on CMU-1.ndpi L0's DQT=6), AC is untouched on both sides. Root cause unknown; see L12 for v0.3+ debugging plan.

## 5. Retired in v0.3

Items closed during the v0.3 polish-and-test push. One line per ID; see
the named commit's message for the full rationale and the test that
locks the change in.

**Limitations (L-prefix):**

- **L1** — SoftwareLine trailing `\r` (`9d19d48`)
- **L3** — `CompressionUnknown` round-trip via patched in-memory TIFF (`b20c5bf`)
- **L7** — NDPI overview MCU detection (folded into the `MCUSizeOf` helper, `ec3aa2a`)
- **L9** — `jpegturbo.Crop` concurrency stress test (`ecbad22`)
- **L10** — SVS LZW label decode-restitch-encode (`76a7dc9`)
- **L11** — SVS associated-image MCU detection (`ec3aa2a`)
- **L15** — Hamamatsu-1.ndpi sparse fixture (`07dadfa`)
- **L16** — Batched parity oracle runner (`0ec51d1`)
- **L18** — `ConcatenateScans` empty-JPEGTables gate (`31e2b42`)

**Reviewer suggestions:**

- **T1** — `NewTestConfig` to `opentile/opentiletest` (`4e3e7f0`)
- **T2** — `withRegistry` test helper (`6576ff8`)
- **T3 + T6** — IFD-chain helper, real 3-IFD walk, cycle rejection (`90fe92e`)
- **T4** — `tiffCompressionToOpentile` exhaustive coverage (`4360b93`)
- **T5** — Aperio CRLF / whitespace / duplicate-key parser tests (`d4586e1`)
- **A1** — `ErrTooManyIFDs` sentinel (`28b75eb`)
- **A2** — `OpenFile` errors include path (`17a391c`)
- **A3** — `Formats() []Format` introspection helper (`af58f63`)
- **A4** — `Config.TileSize` zero-size semantics documented (`0557c49`)
- **O1** — `walkIFDs` bulk-reads each IFD's body (`b8d7a54`)
- **O2** — `int(e.Count)` 32-bit truncation comment (`a08b4a3`)
- **D1, D2, D3** — `decodeASCII` NUL tolerance, `decodeInline` *byteReader rationale, `Metadata.AcquisitionDateTime` IsZero sentinel (`b6b6234`)
- **I1** — Fold zero-length check into `indexOf` (`cd850a0`)
- **I2** — `walkIFDs` overlapping-IFD limit documented (`b6b6234`)
- **I3** — `Scan` reuses caller-supplied `*bufio.Reader` (`761c3e3`)
- **I6** — Ranged RSTn check in `isStandalone` (`8c241a2`)
- **I7** — `ReplaceSOFDimensions` byte-scan invariant documented (`b6b6234`)
- **I8** — `paddedJPEGOnce` uses `sync.Once` (`1e5c367`)

**N-numbered items from the v0.3 plan:**

- **N-1** — Single canonical SOF dimensions patcher (`3a72914`)
- **N-2** — Segment-walker for SOS / DRI lookup (`2f4378c`)
- **N-3** — Cache luma DC per level via `CropOpts` (`108e6da`)
- **N-4** — DQT lookup walks segments before byte-scan (`d40c6d2`)
- **N-5** — `WithNDPISynthesizedLabel(bool)` option (`cf8889d`)
- **N-6** — Chroma DC=0 visual behaviour documented (`b6b6234`)
- **N-7** — `%w` wrapping audit in `formats/ndpi` (`308d5ad`)
- **N-9** — NDPI sniff cross-cutting peek documented (`b6b6234`)

**N-numbered items closed as not-applicable (architecture changed since plan was written):**

- **I4 / N-? (Assembler)** — Plan proposed an `Assembler` type to precompute the JPEG header prefix per level. NDPI striped already amortises the prefix work via per-frameSize header caching in `getPatchedHeader`; SVS associated `Bytes()` is called rarely enough that per-call overhead doesn't show in profiles. The optimisation didn't apply.
- **I5** — Plan proposed a defensive RSTn count check requiring "exactly 1 RSTn per fragment." Aperio SVS strips legitimately carry multiple internal RSTn markers (one per encoder restart-interval row); enforcing exactly-1 would error on valid inputs.
- **N-10** — Plan proposed inverting striped Tile()'s Crop fall-through to choose the helper up front by geometry. The `extendsBeyond` heuristic is broader than "Crop errored," so the inversion changes which tiles take the OOB-fill code path → different bytes via a different libjpeg-turbo transform path. Reverted with an inline comment naming the divergence (`acc2282`).

---

## 6. v0.4 gate outcomes (live)

Tasks 1-4 of the v0.4 plan are JIT verification gates that decide
done-when bars and fix paths for the rest of the milestone. Outcomes
recorded here as they land.

### Task 1 — JP2K determinism gate

- **Date:** 2026-04-26
- **Outcome:** byte-deterministic. Two passes through
  `imagecodecs.jpeg2k_encode` with the upstream Python opentile
  options (level=80, codecformat=J2K, colorspace=SRGB, mct=True,
  reversible=False, bitspersample=8) on the JP2K-33003-1.svs L0
  tile (0,0) produced identical bytes:
  ```
  pass1 sha256=5862056ca6dcbd079403c1f5debd36ce104afe9bbb2fd56ffc5fec2ca7f82080 len=58366
  pass2 sha256=5862056ca6dcbd079403c1f5debd36ce104afe9bbb2fd56ffc5fec2ca7f82080 len=58366
  identical=True
  pixel_equal=True
  ```
- **Consequence:** v0.4 R4 (SVS corrupt-edge reconstruct) targets
  **byte-parity** with Python opentile. Tasks 12 / 16 enforce
  byte-equality assertions; the alternative pixel-equivalent fallback
  is not needed.

### Task 3 — L12 reproduction shape

- **Date:** 2026-04-26
- **Outcome:** **Case D — neither libjpeg-turbo bug nor cgo wrapper bug. The divergence is in our control flow above the wrapper.** The plan offered Cases A/B/C; the actual finding is sharper.

  Determinism check (5 passes each, OS-2.ndpi L5 tile (3,0)):
  - Go side: byte-deterministic (sha `16996cac…6fbc`, 76,418 bytes, 5/5 identical)
  - Python side: byte-deterministic (sha `a1173319…8a24`, 76,307 bytes, 5/5 identical)
  - Each side individually deterministic; the divergence is purely between languages.

  Pixel comparison of the two outputs (after libjpeg decode):
  - Image content (cols 0-895, the in-image region): byte-equal pixel-for-pixel ✓
  - OOB strip (cols 896-1023, image x=3968-4095 past the L5 image's x=3968 right edge):
    - Go: RGB = (128, 128, 128) — mid-gray
    - Python: RGB = (255, 255, 255) — white

  Our `LuminanceToDCCoefficient(luminance=1.0)` returns the correct DC=170 (verified locally on the same OS-2 L5 JPEGHeader). So the cached `dcBackground` is right; we just don't use it on this tile because the dispatch never reaches `CropWithBackgroundLuminanceOpts`.

  Root cause:
  - **Python** (`turbojpeg.py:608-694` `crop_multiple`, gated by `__need_fill_background` at `turbojpeg.py:839-863`) decides geometry-first: route through CUSTOMFILTER iff `crop_region.x + crop_region.w > image_size[0]` (or similar in y) AND luminance ≠ 0.5. Doesn't try plain `Crop` first.
  - **Go** (`formats/ndpi/striped.go::Tile` post-acc2282) tries plain `Crop` first, falls through to `CropWithBackgroundLuminanceOpts` only if `Crop` errors. For OS-2 L5 tile (3,0) and similar edge tiles, `Crop` SUCCEEDS (libjpeg-turbo accepts the MCU-aligned geometry even though it extends past the image). We never call `CropWithBackground`. Plain `Crop`'s default OOB fill is mid-gray (DC=0), not the white-fill DC the cached `dcBackground` would have planted.

  This is the same divergence the v0.3 T30 task uncovered — and reverted (`acc2282`). The revert was the wrong call: T30's geometry-first inversion was the correct fix; the v0.3 fixtures that "broke" were encoding the buggy mid-gray output. Deferred entry L12 currently misframes this as "tjTransform CUSTOMFILTER non-determinism," which is wrong on every count.

- **Consequence:** L12 is a control-flow fix, not a libjpeg-turbo investigation. Two changes needed in v0.4 Task 9:
  1. Replace `frameSize`-based `extendsBeyond` with the **image-size-based** geometry check that matches Python's `__need_fill_background`. Specifically:
     `extendsBeyond := tileXOrigin+l.tileSize.W > l.size.W || tileYOrigin+l.tileSize.H > l.size.H`
     where `tileXOrigin = x * l.tileSize.W` (position in the image, not the assembled frame).
  2. Always route through `CropWithBackgroundLuminanceOpts` when `extendsBeyond` is true, regardless of whether `Crop` would succeed.
  3. Regenerate the OS-2 / Hamamatsu-1 NDPI fixtures with the corrected output (the v0.3 fixtures currently encode the wrong mid-gray fill on edge tiles). Confirm new fixtures byte-match Python via the parity oracle.

  No C-only repro needed; no upstream-bug ticket; no L12 → Permanent demotion.

  **Status (2026-04-26): fixed.** Task 9 landed the dispatch change and regen'd CMU-1 / OS-2 / Hamamatsu-1 NDPI fixtures. Parity oracle removes the L12 carve-out in the same commit; every NDPI tile is now byte-equal to Python opentile.

### Task 2 — NDPI Map fixture audit

- **Date:** 2026-04-26
- **Outcome:** Map pages present on two of three local NDPI fixtures.
  ```
  CMU-1.ndpi          : no Map page (5 pages, max mag = 20.0, min = -1.0)
  OS-2.ndpi           : page 11 (198x580)  mag=-2.0  <-- MAP
  Hamamatsu-1.ndpi    : page  7 (205x600)  mag=-2.0  <-- MAP
  ```
  Upstream confirmation, two layers:
  - **tifffile** (`_series_ndpi`, tifffile.py:5049-5072) DOES classify
    Map pages — `s.name = 'Map'` for `mag == -2.0`. The data is
    structurally exposed at the tifffile layer; a consumer reaches
    them via `TiffFile(path).series[i]` then `pages[0].asarray()`.
  - **Python opentile** (`NdpiTiler` in `ndpi_tiler.py:88-102`) does
    NOT expose Map pages. `_is_label_series` and `_is_thumbnail_series`
    both return False; only `_is_overview_series` (matching "Macro")
    surfaces anything. There's no `_is_map_series` predicate and no
    `tiler.maps` property.
  Surfacing Map pages on the Go side is therefore filling an
  opentile-level scope decision, not inventing a new category — the
  precedent exists at the tifffile layer. Parallels the v0.2 NDPI
  label synthesis (L14).
  Both Map pages on our fixtures are 8-bit grayscale (uint8, 2D
  shape — no third dim). Task 6's `mapPage` implementation should
  byte-passthrough the strip (same pattern as `KindLabel` for SVS
  LZW labels: bytes the source carried, with whatever Compression
  the page advertises). Downstream consumers decoding the Map page
  need to expect a single-channel image, not RGB.
- **Consequence:** L6 / R13 stays in v0.4 scope. Tasks 6-8 proceed
  using OS-2 + Hamamatsu-1 fixtures.

---

## 7. Triage process

Once the branch lands on a remote, every numbered item above should become a tracked issue (GitHub, Linear, etc.) — scope items → roadmap epics, limitations → user-facing docs, reviewer suggestions → individual backlog tickets. Delete entries from this file as they get filed. The goal is for this file to eventually shrink to zero as polish milestones retire each item.
