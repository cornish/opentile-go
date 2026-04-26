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

### L6 — NDPI Map pages (`mag == -2.0`) are silently dropped
- **Source:** NDPI classifier port (post-Batch 4)
- **Severity:** v0.4 — paired with R13. Needs a `"map"` `AssociatedImage` Kind plus a real test fixture (a Hamamatsu slide with a Map page).
- **Detail:** `classifyPage` returns `pageMap` for Magnification tag value -2.0, and `Factory.Open` ignores that kind. Upstream opentile also does not expose Map pages as a first-class associated image.

### L12 — NDPI edge-tile OOB fill is mid-gray, not white (control-flow bug)
- **Source:** NDPI v0.2 architectural rewrite (feat/v0.2); root cause re-diagnosed in v0.4 Task 3 (2026-04-26).
- **Severity:** v0.4 — control-flow fix in `formats/ndpi/striped.go::Tile`. Not a libjpeg-turbo bug, not a CUSTOMFILTER non-determinism, not an entropy-encoding subtlety. The v0.2 + v0.3 framing was wrong on every count; v0.4 Task 3 (`docs/superpowers/plans/2026-04-26-opentile-go-v04.md` §6) records the corrected diagnosis and Task 9 has the fix.

  ⚠️ **The v0.3 `tests/fixtures/OS-2.ndpi.json` and `tests/fixtures/Hamamatsu-1.ndpi.json` fixtures encode the BUGGY mid-gray output.** `TestSlideParity` confirms our (wrong) bytes against committed (wrong) fixture hashes; it does NOT confirm correctness. Until v0.4 Task 9 lands, the parity oracle's NDPI edge-tile `t.Logf` lines are not "documented harmless divergences" — they are real correctness failures masked by the wrong fixtures. Task 9 regenerates these fixtures and removes the parity-oracle carve-out.

- **Detail:** Python opentile (`turbojpeg.py:839-863`'s `__need_fill_background`) decides geometry-first: route through CUSTOMFILTER iff `crop_region.x + crop_region.w > image_size[0]` OR `crop_region.y + crop_region.h > image_size[1]`, AND `background_luminance != 0.5`. No try-Crop-first pattern.

  Our `striped.go::Tile` tries plain `Crop` first, falls through to `CropWithBackgroundLuminanceOpts` only when `Crop` errors. For OS-2 / Hamamatsu-1 edge tiles where `Crop` succeeds despite the tile extending past the image (libjpeg-turbo accepts the MCU-aligned geometry), we silently return `Crop`'s default OOB fill — DC=0 → mid-gray (RGB 128,128,128). Python plants DC=170 → white (RGB 255,255,255).

  Verified on OS-2.ndpi L5 tile (3,0): in-image pixels (cols 0-895) match Python byte-for-byte; OOB strip (cols 896-1023) is mid-gray on Go, white on Python. Both sides individually byte-deterministic; the divergence is purely between languages and exclusively in the OOB region.

  The v0.3 T30 task (`acc2282`) attempted this fix but reverted because the v0.3 fixtures already encoded the buggy mid-gray output — the "regression" was actually the correct behaviour returning. v0.4 Task 9 lands the fix and regenerates the fixtures together.
  Alternatively, submit an upstream issue to libjpeg-turbo asking
  whether CUSTOMFILTER is expected to be deterministic across two
  invocations with identical inputs.

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

### L17 — NDPI label cropH rounded to MCU multiple, not full image height

- **Source:** L7 fix in v0.3 (Task 10) surfaced the divergence
- **Severity:** v0.4 — needs the `CropWithBackground` ragged-height path (luminance + chroma DC math threaded through) before this can land cleanly. The OS-2 and Hamamatsu-1 fixtures regenerate once the fix lands.
- **Detail:** `formats/ndpi.newLabelImage` computes `cropH` as
  `(overview.size.H / mcuH) * mcuH` (rounded down to a whole-MCU multiple)
  to satisfy libjpeg-turbo's `TJXOPT_PERFECT` requirement that crops are
  MCU-aligned. Python opentile's `crop_multiple` tolerates ragged heights
  via its CUSTOMFILTER. Result: when an overview's height is not a multiple
  of its MCU height, our Go label is one MCU row shorter than Python's.

  Visible on OS-2.ndpi (344x392 Go vs 344x396 Python) and Hamamatsu-1.ndpi
  (640x728 Go vs 640x732 Python). CMU-1.ndpi (352x408) is unaffected because
  its image height is divisible by mcuH.

  *Why this is the v0.3 behavior:* fixing it cleanly requires routing
  ragged-height label crops through `CropWithBackground` (with the white-
  fill DC math from Theme 4) rather than the bare `Crop` path. That's a
  multi-step change orthogonal to L7's MCU-detection fix. v0.3 ships with
  the new MCU detection (more correct: CMU-1 now matches Python byte-for-
  byte) and documents this remaining gap.

  *How to fix in v0.4:* detect ragged height in `newLabelImage`, route
  through `CropWithBackground` with luminance=1.0 and chroma DC=0 (matches
  Python). The OS-2 and Hamamatsu-1 fixtures will need re-regeneration
  once the fix lands.

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
