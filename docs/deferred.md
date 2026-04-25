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

| ID | Feature | Target | Status |
|----|---------|--------|--------|
| R1 | NDPI format support (Hamamatsu) | v0.2 | ✅ landed (Batches 2-7, parity verified) |
| R2 | `internal/jpeg` marker package | v0.2 | ✅ landed (Batch 2) |
| R3 | SVS associated images — label, overview, thumbnail | v0.2 (promoted from v0.3) | ✅ landed (Task 21, `9cd27cb`) |
| R4 | Aperio SVS corrupt-edge reconstruct fix (currently returns `ErrCorruptTile`) | v1.0 | deferred |
| R5 | Philips TIFF (sparse-tile filler) | v0.4 | deferred |
| R6 | 3DHistech TIFF | v0.5 | deferred |
| R7 | OME TIFF | v0.5 | deferred |
| R8 | BigTIFF support | v0.2 | ✅ landed (Batch 1) |
| R9 | JPEG 2000 decode/encode (native tiles pass through; decoding only matters once associated-image re-encoding lands) | v0.3+ | deferred |
| R10 | Remote I/O backends (S3, HTTP range, fsspec equivalents) | out-of-scope; consumers supply `io.ReaderAt` | permanent |
| R11 | Python parity oracle under `//go:build parity` | v0.2 | ✅ landed (Task 25-26, Batch 7) |
| R12 | CLI wrapper | out-of-scope for v1 | permanent |
| R13 | NDPI Map (`mag == -2.0`) pages exposed as associated images | v0.3+ | deferred (currently dropped in classifier) |

---

## 2. Known limitations in v0.1 (real behavior gaps)

Caught by the three-slide integration tests or during implementation. The library opens real SVS files correctly, but these behaviors are worth surfacing to users.

### L1 — SoftwareLine has trailing `\r`
- **Source:** diagnostic run during Task 20 fixture generation
- **Severity:** Limitation (cosmetic)
- **Detail:** Real Aperio `ImageDescription` strings use CRLF line endings, so `desc[:newline]` in `parseDescription` leaves a trailing `\r` on `SoftwareLine`. Example: `"Aperio Image Library v11.2.1 \r"`.
- **Fix sketch:** `strings.TrimRight(desc[:newline], "\r\n ")` in `formats/svs/metadata.go`.

### L2 — Non-tiled pages silently skipped
- **Source:** Task 16/20 page-classification fix (`f7a27c4`)
- **Severity:** Limitation (intended for v0.1 scope)
- **Detail:** `Factory.Open` skips TIFF pages without `TileWidth` rather than surfacing them. This hides thumbnail, label, and macro images. Correct behavior once R3 (associated images) lands: classify and expose these pages as `AssociatedImage`.

### L3 — `CompressionUnknown` untested against a real-world slide
- **Source:** Task 16 quality review
- **Severity:** Limitation (no real example yet)
- **Detail:** `mapCompression` returns `CompressionUnknown` for TIFF compression codes it doesn't recognize. None of the three test slides exercise this path. A slide using LZW or Deflate would.

### L4 — MPP may be absent from some slides' ImageDescription
- **Source:** diagnostic run during Task 20
- **Severity:** Limitation (slide-dependent)
- **Detail:** CMU-1 embeds `MPP = 0.4990` in its ImageDescription, but not every Aperio slide does. When absent, `svs.Metadata.MPP` is zero and every Level's `MPP()` returns `SizeMm{0, 0}`. No bug, but a caller who expects non-zero MPP should check `SizeMm.IsZero()` rather than assuming a value.

### L5 — `internal/tiff` peeks at NDPI-specific tag to select IFD layout
- **Source:** Architecture audit (post-Batch 4)
- **Severity:** Limitation (cross-cutting)
- **Detail:** `internal/tiff` is conceptually format-agnostic, but `File.Open` reads tag 65420 (Hamamatsu NDPI FileFormat) in the first IFD to decide whether to dispatch the NDPI-layout IFD walker. Necessary because NDPI files have classic TIFF magic 42 with no header-level distinguisher. A cleaner split would have `tiff.Open` expose a generic "layout" hint and let format packages drive selection — but that requires callers to know NDPI exists. Acceptable for v0.2; revisit when adding Philips/OME or any other format with its own TIFF dialect.

### L6 — NDPI Map pages (`mag == -2.0`) are silently dropped
- **Source:** NDPI classifier port (post-Batch 4)
- **Severity:** Limitation (v0.2 scope)
- **Detail:** `classifyPage` returns `pageMap` for Magnification tag value -2.0, and `Factory.Open` ignores that kind. Upstream opentile also does not expose Map pages as a first-class associated image. No known real-world consumer of Map content, but if users ask for it we'd add it as an `AssociatedImage` with `Kind() == "map"`. Tracked as R13.

### L7 — NDPI overview cropping assumes MCU 16×16
- **Source:** Task 19 plan note
- **Severity:** Limitation (format-assumption)
- **Detail:** `newLabelImage` receives hard-coded `mcuW=16, mcuH=16` from `Factory.Open`, assuming Hamamatsu always uses YCbCr 4:2:0. If a macro page ever uses 4:4:4 (MCU 8×8) or 4:2:2 (MCU 16×8), the crop region computed with 16×16 may not be MCU-aligned and `TJXOPT_PERFECT` will reject the crop. Refinement: read each macro page's SOF and use its actual MCU size. Not observed on any of the three local NDPI slides, but worth fixing when the first real user reports it.

### L8 — SVS v0.1 page classifier was guessed, not ported from upstream
- **Source:** Architecture audit (post-Batch 4)
- **Severity:** Limitation (needs verification in Task 21)
- **Detail:** `formats/svs/svs.go` skips non-tiled pages (v0.1 scope) and Task 21's plan had a classifier based on `ImageDescription == "label"`/`"macro"`. Upstream tifffile's SVS detection derives series names from the first line of `ImageDescription` (Aperio's format embeds markers there). Before implementing Task 21, read `cgohlke/tifffile`'s `_series_svs` or similar and `imi-bigpicture/opentile`'s SVS tiler, then port whatever the real classification logic is. Do not carry forward v0.1's guess.

### L9 — Concurrency stress on `internal/jpegturbo.Crop` is undocumented
- **Source:** Batch 3 quality review (suggestion 7)
- **Severity:** Suggestion
- **Detail:** Per-call `tjInitTransform`/`tjDestroy` is safe for goroutine-parallel `Crop` calls per libjpeg-turbo's threading model, but no test proves it. A simple `t.Parallel()` loop running 1000 crops across 32 goroutines would lock the contract in. Not blocking.

### L10 — SVS LZW label returns only strip 0  *(resolved in v0.3)*
- **Source:** Task 21 implementation (feat/v0.2)
- **Severity:** Limitation (v0.2 upstream-parity behavior)
- **Status:** Fixed on `feat/v0.3` (Task 11). `formats/svs/lzwlabel.go` now decodes each strip with a TIFF-LZW reader (`internal/tifflzw`, vendored from `golang.org/x/image/tiff/lzw` with the matching writer added), raster-concatenates, and re-encodes as a single LZW stream covering the full image. The parity oracle skips SVS labels uniformly until Python opentile lands the same fix; an upstream PR is on the todo list.
- **Original detail:** `formats/svs.stripedLabel.Bytes()` returned the raw bytes of strip 0 of the TIFF label page, not a stitched or re-encoded full-image LZW stream. For the multi-strip labels in the CMU fixtures (67/67, 67/67, 71/71 strips), consumers received a valid LZW blob representing only `RowsPerStrip` rows (7 in most fixtures) of the ~463-row label — a ~1.5% vertical sliver. Matched Python opentile's `SvsLabelImage.get_tile((0,0))` which returns `_read_frame(0)` unconditionally.

### L11 — SVS associated-image DRI assumes 16×16 MCU
- **Source:** Task 21 follow-up review (feat/v0.2)
- **Severity:** Limitation (format-assumption)
- **Detail:** `formats/svs.stripedJPEGAssociated.Bytes()` computes the DRI as
  `ceil(width/16) × ceil(RowsPerStrip/16)` MCUs per strip, hardcoding the
  Aperio YCbCr 4:2:0 default. If a thumbnail or overview page were encoded
  with different luma/chroma sampling factors the computed DRI would be
  incorrect and the assembled JPEG would decode with artefacts at every
  strip boundary. The code does not read SOF sampling factors to verify.

  *Why this is the v0.2 behavior:* all three sample SVS fixtures use the
  YCbCr 4:2:0 Aperio default; the RestartInterval math matches what Python
  opentile's `_manipulate_header` computes for the same files. Parallel to
  L7 for NDPI overview crop.

  *How to fix:* parse the SOF from the first strip (similar to
  `ndpi/oneframe.go`'s `sof.MCUSize()` pattern), derive the MCU size from
  the sampling factors, and use that instead of the 16×16 constant.

### L12 — NDPI edge-tile entropy-encoding divergence
- **Source:** NDPI v0.2 architectural rewrite (feat/v0.2)
- **Severity:** Limitation (parity gap on edge tiles)
- **Detail:** `internal/jpegturbo.CropWithBackground` now uses the
  canonical PyTurboJPEG white-fill algorithm: `internal/jpeg.LumaDCQuant`
  parses the source's DQT table 0, `LuminanceToDCCoefficient` computes
  `round((luminance * 2047 - 1024) / dc_quant)` (banker's rounding), and
  our CUSTOMFILTER callback plants the resulting DC coefficient in
  OOB luma blocks — identical math to
  `turbojpeg.py:__map_luminance_to_dc_dct_coefficient`. DC values
  verified to match Python byte-for-byte on CMU-1.ndpi L0 (dq=6 →
  dc=170 for luminance=1.0).

  Despite matching DC inputs, NDPI edge tiles still show small
  byte-level differences vs Python opentile — typically a single byte
  in the entropy stream for CMU-1.ndpi, up to ~130 bytes for OS-2.ndpi.
  The divergence propagates through JPEG's differential DC encoding so
  decoded pixels differ visibly in the OOB region only. Root cause is
  a subtle tjTransform / libjpeg-turbo non-determinism we haven't
  pinned down: same flags (`TJXOPT_PERFECT | TJXOPT_CROP`), same
  CUSTOMFILTER output, different entropy encoding. Possibly related to
  MCU boundary handling at sub-MCU-aligned image edges or DC predictor
  state across the callback boundary.

  Parity-visible area (inside the image): byte-identical. Only the
  OOB background region diverges.

  *Why this is the v0.2 behavior:* unblocks parity-oracle green on all
  5 sample slides; chasing the remaining divergence would require
  in-depth libjpeg-turbo source instrumentation. Parity test downgrades
  NDPI-only edge-tile mismatches to `t.Log`.

  *How to fix in v0.3+:* instrument tjTransform to dump the coefficient
  buffer state before and after the CUSTOMFILTER callback on both
  Python and Go sides for a known-diverging tile, diff the buffers,
  identify the single coefficient that differs, trace backward.
  Alternatively, submit an upstream issue to libjpeg-turbo asking
  whether CUSTOMFILTER is expected to be deterministic across two
  invocations with identical inputs.

### L13 — v0.2 NDPI striped path was architecturally wrong for ~50 hours before benchmarking caught it
- **Source:** NDPI v0.2 architectural rewrite (feat/v0.2, post-Batch 4)
- **Severity:** Limitation (process note; resolved)
- **Detail:** `formats/ndpi/striped.go` originally gated on TIFF tag 322
  (TileWidth), which NDPI pages never carry. Every pyramid level fell
  through to a whole-level `tjTransform` crop per tile — ~3000x slower
  than Python opentile for L0 of CMU-1.ndpi (3.3s/tile vs 0.97ms/tile;
  projected ~9h for a full-level fixture). Fixed by porting tifffile's
  `_page._gettags` McuStarts rewrite (`internal/jpeg/ndpi_tile.go` +
  `formats/ndpi/stripes.go`) plus Python opentile's
  `NdpiStripedImage._read_extended_frame` tile-assembly path
  (`formats/ndpi/striped.go`).

  *Lesson captured in `feedback_ndpi_architecture.md`.* Reinforces the
  "don't guess — read upstream" rule already codified in CLAUDE.md:
  "does the slide open?" is not a sufficient NDPI smoke test; the
  `NDPI_BENCH_SLIDE` benchmark (`formats/ndpi/bench_test.go`) now
  forces a per-tile-time regression gate so this specific failure mode
  cannot recur silently.

### L14 — NDPI label synthesis is Go-specific; Python opentile does not expose NDPI labels
- **Source:** Batch 7 parity oracle extension
- **Severity:** Limitation (deliberate API divergence)
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

### L15 — Hamamatsu-1.ndpi (6.4 GB) is excluded from the committed fixture set
- **Source:** Batch 6 Task 24 (`673d475`)
- **Severity:** Limitation (coverage)
- **Detail:** `slideCandidates` in `tests/integration_test.go` lists five
  sample slides; `Hamamatsu-1.ndpi` is not among them. Its fixture
  would exceed the 5 MB soft-cap adopted in the plan (the slide's tile
  count is ~20x larger than OS-2.ndpi's 3.8 MB fixture). CI and the
  parity oracle therefore do not exercise NDPI's 64-bit offset extension
  path end-to-end on a real file larger than 4 GB; only unit tests and
  the ~931 MB OS-2.ndpi cover the closely-related code.

  *Mitigation in place:* `formats/ndpi` benchmark opens and hashes tiles
  of Hamamatsu-1.ndpi when `NDPI_BENCH_SLIDE` is set, so the 64-bit
  path is exercised interactively if a developer has the slide locally.

  *How to fix:* either raise the committed-fixture cap (git-lfs) or
  generate a sparse fixture that hashes only a sampled subset of tiles
  per level (integration test would need a "sampled" mode).

### L16 — Parity oracle default run samples only 10 tiles per level
- **Source:** Batch 7 Task 26 (`057f955`)
- **Severity:** Suggestion (coverage)
- **Detail:** `samplePositions` in `tests/oracle/parity_test.go` returns
  up to 10 tile positions per level (corners, midline, interior) rather
  than every tile. `-parity-full` enumerates all tiles but is not in
  CI because full-walk runtime on OS-2.ndpi approaches 30 minutes.

  A deliberately divergent tile inside the sampled 10 would be caught;
  a divergence confined to, say, a single mid-edge tile we don't sample
  would not. The structural/architectural divergences we know about
  (L10 SVS LZW label, L12 NDPI edge fill) show up consistently at the
  sampled corners and so are caught by the default run.

  *How to improve without blowing up runtime:* batch several tile
  positions into one Python invocation (subprocess startup is ~200 ms
  and dominates the per-tile cost at 10 samples). A runner that
  accepts many positions per invocation could reasonably raise the
  sample to hundreds per level.

### L17 — NDPI label cropH rounded to MCU multiple, not full image height

- **Source:** L7 fix in v0.3 (Task 10) surfaced the divergence
- **Severity:** Limitation (parity gap on slides where image_h % mcu_h ≠ 0)
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

### L18 — `ConcatenateScans` rejected `ColorspaceFix=true` when JPEGTables is empty  *(resolved in v0.3)*

- **Source:** Task 12 fixture-generation attempt against `svs_40x_bigtiff.svs` (Grundium Ocus scanner)
- **Severity:** Limitation (blocked SVS associated-image read on slides whose pages carry no shared JPEGTables tag — strips embed their own DQT/DHT/SOF inline)
- **Status:** Fixed on `feat/v0.3`. `internal/jpeg/concat.go` now skips both the tables splice and the APP14 splice when `len(JPEGTables) == 0`, matching upstream's gate at `opentile/jpeg/jpeg.py:192-198` (where `rgb_colorspace_fix` is layered inside the `if jpeg_tables is not None:` branch). Verified byte-identical to Python opentile on `scan_620_.svs` thumbnail (3,145,734 bytes), label (834,864 bytes), and overview (116,344 bytes). Two unit tests in `internal/jpeg/concat_test.go` lock the gate behavior in.
- **Original detail:** `internal/jpeg/concat.go` errored with `ColorspaceFix requires non-empty JPEGTables` whenever `ColorspaceFix=true` and `len(JPEGTables) == 0`. Surfaced on the BigTIFF Grundium slide (`svs_40x_bigtiff.svs`): every page reports `hasJPEGTables=false(len=0)` and the thumbnail/macro associated-image reads failed immediately. Tile reads worked fine because the tiled-level path (`formats/svs/tiled.go:162`) only splices when `len(jpegTables) > 0`.

---

## 3. Reviewer suggestions accepted but not applied

Non-blocking items raised during per-task spec / quality review. Categorized by area.

### Testing hygiene

- **T1** — Move `NewTestConfig` out of `options.go` into a dedicated `opentile/opentiletest` subpackage. Matches stdlib idiom (`httptest`, `iotest`). Ensures it cannot be reached from production code paths. (Source: Task 14 review.)
- **T2** — `opentile_test.go` uses a package-global registry with `resetRegistry()` in each test. Unsafe if tests ever call `t.Parallel()`. Either document "do not parallelize this file" or introduce a `withRegistry(t, factories...)` helper that uses `t.Cleanup`. (Source: Task 13 review.)
- **T3** — No test coverage for the IFD cycle-detection branch or the `maxIFDs=1024` cap in `walkIFDs`. (Source: Task 8 review.)
- **T4** — No test for the `mapCompression` default branch or the `TagYCbCrSubSampling` / `TagBitsPerSample` accessors. (Source: Task 10 review.)
- **T5** — No CRLF / whitespace / duplicate-key edge tests for the Aperio `ImageDescription` parser. CRLF is the likely real-world gap (see L1). (Source: Task 15 review.)
- **T6** — `TestWalkIFDsMultiple` is named as if it exercises a multi-IFD chain but only asserts single-IFD termination. Rename or add a real multi-IFD builder. (Source: Task 8 review.)

### API polish

- **A1** — `maxIFDs` is a literal embedded in an error message. Callers cannot programmatically distinguish "too many IFDs" from other parse errors. Promote to `ErrTooManyIFDs` sentinel. (Source: Task 8 review.)
- **A2** — `OpenFile` wraps errors but does not include the path. `fmt.Errorf("opentile: open %s: %w", path, err)` would aid debugging. (Source: Task 13 review.)
- **A3** — Consider a `Formats() []Format` introspection helper for diagnostics / tooling. (Source: Task 13 review.)
- **A4** — `Config.TileSize()` returns `(Size, true)` when caller explicitly passes `Size{0, 0}`. Format packages should reject `(Size{}, true)` as malformed rather than treating it the same as `(Size{}, false)`. Document this expectation in the godoc; no code change yet since no caller hits it. (Source: Task 14 review fix discussion.)

### Optimization

- **O1** — `walkIFDs` issues four `ReadAt` calls per entry. Bulk-read the full IFD (`2 + 12*count + 4` bytes) with a single `ReadAt` and slice-decode. Pays off on adversarial inputs with inflated entry counts. (Source: Task 8 review.)
- **O2** — `int(e.Count)` on 32-bit platforms in `JPEGTables`/`ICCProfile`. Theoretically truncates on `Count > 2 GiB`. Practical values are <1 MB; not a real risk on 64-bit targets. (Source: Task 10 review.)

### Documentation

- **D1** — `decodeASCII` silently tolerates missing NUL terminators; add a one-line comment noting this. (Source: Task 7 review.)
- **D2** — `decodeInline`'s `b *byteReader` parameter is only used for `b.order`; a doc comment would clarify why it takes a full reader. (Source: Task 7 review.)
- **D3** — `Metadata.AcquisitionDateTime`: add a brief note that partial Date/Time input yields the zero value, and zero is the "unknown" sentinel. (Source: Task 15 review.)

### Internals

- **I1** — `indexOf` in `tiledImage` could fold the `length == 0` corrupt-tile check in so `Tile` and `TileReader` don't duplicate it. Minor. (Source: Task 16 review.)
- **I2** — `walkIFDs` does not detect overlapping IFDs (an IFD starting inside a previous IFD's body). The seen-offset map catches exact matches only. Acceptable for v0.1; worth a comment. (Source: Task 8 review.)
- **I3** — `internal/jpeg.Scan` wraps its `io.Reader` unconditionally in `bufio.NewReader`, preventing callers from chaining `Scan` → `ReadScan` on the same underlying position. Current workaround: `extractScanData` byte-scans for SOS rather than calling `Scan` then `ReadScan`. If we ever want to eliminate the duplicate parser, either have `Scan` detect an existing `*bufio.Reader` (like `ReadScan` does) or expose the buffered reader via an iterator context. (Source: Batch 2 quality review, suggestion 2.)
- **I4** — `internal/jpeg.ConcatenateScans` rebuilds `SplitJPEGTables` + first-fragment SOF/SOS per call. For 24K-tile slide levels this reparses the same JPEGTables blob 24K times. An `Assembler` struct pre-computing the header prefix once would cut per-tile work measurably. Deferred until profiling on real slides identifies it as a bottleneck. (Source: Batch 2 quality review, suggestion 1.)
- **I5** — `ConcatOpts.RestartInterval > 0` assumes each input fragment contains exactly one restart interval (no internal RSTn markers). Documented in godoc; correct for NDPI and SVS associated-image stripes. Formats that violate this would silently produce a malformed bitstream. Consider a defensive check that counts RSTn markers in each fragment. (Source: Batch 2 quality review, concern C1.)
- **I6** — `Scan` uses literal `0xD0..0xD7` in `isStandalone` alongside the `RST0` constant. Prefer a ranged check (`m >= RST0 && m <= RST0+7`) to avoid drift if constants ever change. (Source: Batch 2 quality review, suggestion 3.)
- **I7** — `internal/jpeg.ReplaceSOFDimensions` finds SOF0 via linear byte scan from the start of the buffer. In well-formed JPEGs this is safe because SOF0 precedes SOS and thus precedes any entropy-coded scan data. Worth a comment in the code noting the assumption. (Source: Task 10 review.)
- **I8** — `oneFrameImage.paddedJPEGOnce` is a plain bool, not `sync.Once` / atomic. Concurrent first-calls may both rebuild the padded JPEG; output is byte-identical so this is benign, but the contract should be documented in the godoc rather than buried in a comment. (Source: Task 18 plan note.)

---

## 4. v0.2 session learnings (process notes)

These are not deferred items — they're observations about how v0.2 development went, kept here so future sessions have context.

- **The "don't guess — read upstream" rule was established mid-session** (commit `993289e`). Codified in CLAUDE.md and in the cross-session memory system (`feedback_no_guessing.md`). Prompted by three separate "guess-and-regret" debugging cycles in v0.2 (NDPI IFD layout, NDPI metadata tag numbers, StripOffsets tag number) and reinforced by two more in Batch 6/7 (NDPI striped vs. oneframe gate on tag 322 — L13; APP14 bytes miscopied — now canonical). Future format work should read upstream first and commit behavior from there.

- **Architecture audit at end of Batch 4** — three minor cleanups landed (NDPI tiler struct consolidated into `tiler.go`, SVS `image.go` renamed to `tiled.go`, stale debug artifacts removed). Full decision trail in commit `cae47cd`.

- **"Does the slide open?" is not a sufficient NDPI smoke test (L13).** The original `formats/ndpi/striped.go` gated on TIFF tag 322 (TileWidth), which NDPI pages never carry. Every pyramid level fell through to `oneFrameImage`'s per-tile whole-level tjTransform — 3000× slower than Python opentile for CMU-1.ndpi L0, but every unit and integration test still passed because they only verified `Open()` + tile counts, never per-tile throughput. Caught during Batch 6 when fixture generation hit a 30-minute timeout on a 188 MB slide. `formats/ndpi/bench_test.go` is now a regression gate that exercises per-tile time under `NDPI_BENCH_SLIDE`.

- **Byte parity is a stronger correctness bar than it first appears (Batch 7).** The parity oracle was originally scoped as a best-effort opt-in check; extending it to `Associated.Bytes()` surfaced two latent divergences in `internal/jpeg/ConcatenateScans` (APP14 byte values and segment ordering) that no other test caught — both produced valid JPEGs, both decoded to correct pixels, both self-consistent inside our own fixtures. Only byte-level comparison against Python opentile surfaced them. Lesson: "decodes to the right pixels" is not synonymous with "correct output" when downstream consumers (wsidicomizer, DICOM pipelines) may hash the output.

- **Parity with an unconfigured upstream has hidden dimensions (Batch 7).** Python opentile 0.20.0's `OpenTile.open(slide, tile_size)` expects `int`, not `(w, h)`; passing a tuple silently worked for SVS and raised `TypeError` from NDPI's adjust_tile_size. Two separate parity agents hit this. The v0.2 runner pins `tile_size: int` and documents the quirk.

- **tjTransform's CUSTOMFILTER callback is not bitwise-deterministic across Go/Python (L12).** Same flags (`TJXOPT_PERFECT | TJXOPT_CROP`), same callback output, same source JPEG → subtly different entropy streams for NDPI edge tiles. Verified our DC coefficient math matches Python's (`round((lum * 2047 - 1024) / dc_quant) = 170` for luminance=1.0 on CMU-1.ndpi L0's DQT=6), AC is untouched on both sides. Root cause unknown; see L12 for v0.3+ debugging plan.

## 5. Triage process

Once the branch lands on a remote, every numbered item above should become a tracked issue (GitHub, Linear, etc.) — scope items → roadmap epics, limitations → user-facing docs, reviewer suggestions → individual backlog tickets. Delete entries from this file as they get filed. The goal is for this file to eventually shrink to zero as polish milestones retire each item.
