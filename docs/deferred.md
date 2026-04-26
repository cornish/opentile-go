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

## 2. Active limitations (open in v0.3 / v0.4+)

Real behaviour gaps that have not been closed. Each entry's **Severity**
field declares whether it's a permanent design choice (the library
cannot reasonably address it without breaking compatibility) or a
v0.4+ punt (a real fix is queued but out of scope for v0.3). Items
closed during v0.3 are listed in §6.

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
- **Severity:** v0.4+ — tracked as R13. No known consumer is asking for Map content; landing this requires a "map" `AssociatedImage` Kind plus a real test fixture.
- **Detail:** `classifyPage` returns `pageMap` for Magnification tag value -2.0, and `Factory.Open` ignores that kind. Upstream opentile also does not expose Map pages as a first-class associated image.

### L12 — NDPI edge-tile entropy-encoding divergence
- **Source:** NDPI v0.2 architectural rewrite (feat/v0.2)
- **Severity:** v0.4+ — tjTransform CUSTOMFILTER non-determinism. Pixel output is visually equivalent but a handful of bytes per edge tile differ between Go and Python; root cause not pinned down.
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
- **Severity:** v0.4+ — needs the `CropWithBackground` ragged-height path (luminance + chroma DC math threaded through) before this can land cleanly.
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

## 6. Triage process

Once the branch lands on a remote, every numbered item above should become a tracked issue (GitHub, Linear, etc.) — scope items → roadmap epics, limitations → user-facing docs, reviewer suggestions → individual backlog tickets. Delete entries from this file as they get filed. The goal is for this file to eventually shrink to zero as polish milestones retire each item.
