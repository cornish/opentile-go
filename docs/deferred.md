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
| R1 | NDPI format support (Hamamatsu) | v0.2 | ✅ in flight on feat/v0.2 |
| R2 | `internal/jpeg` marker package | v0.2 | ✅ landed (Batch 2) |
| R3 | SVS associated images — label, overview, thumbnail | v0.2 (promoted from v0.3) | in flight (Task 21) |
| R4 | Aperio SVS corrupt-edge reconstruct fix (currently returns `ErrCorruptTile`) | v1.0 | deferred |
| R5 | Philips TIFF (sparse-tile filler) | v0.4 | deferred |
| R6 | 3DHistech TIFF | v0.5 | deferred |
| R7 | OME TIFF | v0.5 | deferred |
| R8 | BigTIFF support | v0.2 | ✅ landed (Batch 1) |
| R9 | JPEG 2000 decode/encode (native tiles pass through; decoding only matters once associated-image re-encoding lands) | v0.3+ | deferred |
| R10 | Remote I/O backends (S3, HTTP range, fsspec equivalents) | out-of-scope; consumers supply `io.ReaderAt` | permanent |
| R11 | Python parity oracle under `//go:build parity` | v0.2 | in flight (Batch 7) |
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

### L10 — SVS LZW label returns only strip 0
- **Source:** Task 21 implementation (feat/v0.2)
- **Severity:** Limitation (v0.2 upstream-parity behavior)
- **Detail:** `formats/svs.stripedLabel.Bytes()` returns the raw bytes of strip 0 of the TIFF label page, not a stitched or re-encoded full-image LZW stream. For the multi-strip labels in the CMU fixtures (67/67, 67/67, 71/71 strips), this means consumers receive a valid LZW blob representing only `RowsPerStrip` rows (7 in most fixtures) of the ~463-row label — a ~1.5% vertical sliver.

  *Why this is the v0.2 behavior:* matches Python opentile's `SvsLabelImage.get_tile((0,0))` which returns `_read_frame(0)` unconditionally. Parity with upstream is the v0.2 correctness bar; stitching here would diverge from the parity oracle (Task 25-26) and from `wsidicomizer`'s expected byte stream.

  *How to fix in v0.3:* decode each strip via a TIFF-aware LZW codec (Go stdlib `compress/lzw` with `Order=MSB`, or a ported tifffile codec), raster-concatenate, re-encode as a single LZW stream covering the full image height, and rewrite the LZW-specific TIFF metadata to match. Requires a full LZW codec path. Also update Python opentile with the same fix so the parity oracle continues to compare like for like, or gate the oracle on non-label images.

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

- **The "don't guess — read upstream" rule was established mid-session** (commit `993289e`). It's codified in CLAUDE.md and in the cross-session memory system (`feedback_no_guessing.md`). The rule was prompted by three separate "guess-and-regret" debugging cycles in v0.2: NDPI IFD layout, NDPI metadata tag numbers, and the StripOffsets tag number. Each was eventually fixed by reading the relevant upstream source (tifffile / opentile) directly. Future format work should budget time to read upstream FIRST and commit behavior from there.
- **Architecture audit at end of Batch 4** (this doc) — three minor cleanups landed (NDPI tiler struct consolidated into `tiler.go`, SVS `image.go` renamed to `tiled.go`, stale debug artifacts removed). Full decision trail in the commit that introduced this section.

## 5. Triage process

Once the branch lands on a remote, every numbered item above should become a tracked issue (GitHub, Linear, etc.) — scope items → roadmap epics, limitations → user-facing docs, reviewer suggestions → individual backlog tickets. Delete entries from this file as they get filed. The goal is for this file to eventually shrink to zero as polish milestones retire each item.
