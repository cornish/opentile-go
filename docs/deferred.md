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

**v0.4 scope (final):** close every real bug and parity gap on SVS and NDPI that we have a fixture to drive. Landed: L12 (NDPI edge-tile OOB fill), L17 (NDPI label cropH), L6 / R13 (NDPI Map pages). Permanent items (L4, L5, L14) stay documented as design choices.

**v0.6+ scope (revised on 2026-04-26 after the v0.5 ship):** OME TIFF
in v0.6 closes the last upstream-opentile format. After that we venture
beyond upstream's coverage starting with **Ventana BIF** in v0.7 — a
clinically-common Roche / Ventana iScan format that openslide reads but
upstream opentile doesn't. **Leica SCN** and **Generic Tiled TIFF** are
tentative v0.8+ candidates, gated on real-slide demand. **3DHistech
TIFF** (R6) and **Sakura SVSlide** (R15) are parked behind GH issues —
3DHistech is a niche MRXS conversion target we've never encountered in
the wild; Sakura is rare enough to follow the same trigger-driven
deferral as R4/R9.

The methodology shift starting in v0.7: opentile-go leaves the
"port-from-upstream-opentile-byte-for-byte" pattern. Openslide is the
practical reference for BIF / SCN / Generic TIFF, but it's LGPL 2.1, so
we read it for understanding rather than direct port; correctness bar
shifts from byte-parity to pixel-equivalence with openslide on decoded
tiles.

R4 / R9 (SVS corrupt-edge reconstruct + JP2K decode/encode) remain
parked at [#1](https://github.com/cornish/opentile-go/issues/1) until a
real slide motivates the work.

| ID | Feature | Target | Status |
|----|---------|--------|--------|
| R1 | NDPI format support (Hamamatsu) | v0.2 | ✅ landed (Batches 2-7, parity verified) |
| R2 | `internal/jpeg` marker package | v0.2 | ✅ landed (Batch 2) |
| R3 | SVS associated images — label, overview, thumbnail | v0.2 (promoted from v0.3) | ✅ landed (Task 21, `9cd27cb`) |
| R4 | Aperio SVS corrupt-edge reconstruct fix (currently returns `ErrCorruptTile`) | v0.5+ | deferred — see [#1](https://github.com/cornish/opentile-go/issues/1). Originally promoted to v0.4; demoted on 2026-04-26 because none of our local SVS slides exhibit corrupt edges and 12 tasks of cgo + Pillow-port work to deliver a synthetic-fixture-only feature isn't completeness, it's speculation. Issue captures the full upstream algorithm + Go-side dependency tree; trigger to take it on is a real slide that fails on us with `ErrCorruptTile`. |
| R5 | Philips TIFF (sparse-tile filler) | v0.5 | ✅ landed (commits `1ad463c..7e7bde0`, parity verified across 4 fixtures) |
| R6 | 3DHistech TIFF | parked | parked behind GH issue (TBD). MRXS conversion target produced by 3DHistech software; rare in practice. Trigger to take it on is a real slide. Upstream opentile has a ~200 LOC reader; cheap to revive if motivated. |
| R7 | OME TIFF | v0.6 | next milestone. Closes the upstream-opentile format set. Uses sub-IFDs for pyramid levels rather than top-level IFDs (pattern hint surfaced from the v0.5 spec's "wrapper-page pattern" forward-looking note). |
| R8 | BigTIFF support | v0.2 | ✅ landed (Batch 1) |
| R9 | JPEG 2000 decode/encode (currently passes through native tiles; decode matters for associated-image re-encoding and corrupt-tile reconstruct) | v0.5+ | deferred — see [#1](https://github.com/cornish/opentile-go/issues/1). Only consumer is R4; deferred together. Native JP2K tile passthrough (the v0.1+ behaviour) continues to work — decode is only needed for the reconstruct chain. |
| R10 | Remote I/O backends (S3, HTTP range, fsspec equivalents) | out-of-scope; consumers supply `io.ReaderAt` | permanent |
| R11 | Python parity oracle under `//go:build parity` | v0.2 | ✅ landed (Task 25-26, Batch 7) |
| R12 | CLI wrapper | out-of-scope for v1 | permanent |
| R13 | NDPI Map (`mag == -2.0`) pages exposed as associated images | v0.4 | ✅ landed (commit `7ac3f88`, paired with L6). `Tiler.Associated()` now exposes `Kind() == "map"` entries on slides that carry them. |
| R14 | Ventana BIF (Roche / iScan) | v0.7 | first format beyond upstream opentile's coverage. BigTIFF-based; openslide has a reader (LGPL 2.1, read-for-understanding only). Local fixtures already present in `sample_files/ventana-bif/`. Correctness bar: pixel-equivalence with openslide on decoded tiles (no byte-stable Python reference). |
| R15 | Sakura SVSlide | parked | parked behind GH issue (TBD). Rare format; openslide reads it. Trigger-driven deferral. |
| R16 | Leica SCN | v0.8 (tentative) | BigTIFF-based; common in research microscopy. Openslide reader as reference. Decide based on real-slide demand. |
| R17 | Generic Tiled TIFF | v0.8+ (tentative) | catch-all fallback for unknown vendors with standard TIFF tile layout. Decide based on real-slide demand and whether end users are hitting `ErrUnsupportedFormat` on standards-compliant slides. |

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

## 6. Retired in v0.4

Items closed during the v0.4 existing-format completeness milestone.
One line per ID; named commit's message has the full rationale and
the test that locks the change in.

**Limitations (L-prefix):**

- **L6** — NDPI Map pages (`mag == -2.0`) surface as `AssociatedImage` with `Kind() == "map"` (`7ac3f88`, paired with R13). OS-2.ndpi page 11 and Hamamatsu-1.ndpi page 7 exposed as 8-bit grayscale uncompressed strip-passthrough; CMU-1.ndpi unchanged (no Map page). Deliberate Go-side extension paralleling L14 — Python opentile does not expose Map pages even though tifffile classifies them as `series.name == 'Map'`.
- **L12** — NDPI edge-tile OOB fill (`f4c647b`). Control-flow bug, not the "tjTransform CUSTOMFILTER non-determinism" the v0.2/v0.3 entry claimed. `formats/ndpi/striped.go::Tile` now dispatches geometry-first against image size (matching Python's `__need_fill_background` gate at `turbojpeg.py:839-863`); pre-v0.4 it tried plain `Crop` first and silently returned mid-gray OOB fills on edge tiles where Crop happened to succeed. CMU-1 + OS-2 + Hamamatsu-1 NDPI fixtures regenerated; parity oracle's L12 `t.Logf` carve-out removed in the same commit. Every NDPI tile now byte-equal to Python opentile.
- **L17** — NDPI label cropH passes full image height (`bdaa44e`). One-line `cropH` change in `formats/ndpi/associated.go::newLabelImage` plus dropping the now-unused `mcuH` argument. Matches Python's `_crop_parameters[3] = page.shape[0]` at `ndpi_image.py:144`; libjpeg-turbo's `TJXOPT_PERFECT` accepts the partial last MCU row when the crop ends at the image edge. The pre-v0.4 entry's "needs CropWithBackground" advice was wrong (incomplete read of upstream).

**Roadmap items (R-prefix):**

- **R13** — NDPI Map pages exposed as associated images (`7ac3f88`, paired with L6).

**JIT verification gates (Tasks 1-4 of the v0.4 plan):**

- **T1** — JP2K determinism gate (`c27d9f8`): byte-deterministic. R4's done-when bar (when it lands) is byte-parity with Python opentile.
- **T2** — NDPI Map fixture audit (`e6bbcd5`): OS-2 + Hamamatsu-1 carry Map pages; tifffile classifies them, Python opentile chooses not to surface them. Validated the L6 / R13 path.
- **T3** — L12 reproduction shape (`1b651bf`): Case D — control-flow bug in our Go-side dispatch (the plan offered Cases A/B/C; the actual finding was sharper). Drove the L12 fix above.
- **T4** — R4 mechanism audit (`d2fe107`): port notes at `docs/superpowers/notes/2026-04-26-svs-reconstruct-port.md`. Identified that R4 pulls in 4 new pieces of cgo + Pillow-port infrastructure for a feature whose only test would be synthetic. Drove the v0.4 → v0.5+ deferral of R4 / R9 to issue [#1](https://github.com/cornish/opentile-go/issues/1).

**Deferred (not retired, but resolved by punting to a tracked issue):**

- **R4** (SVS corrupt-edge reconstruct) and **R9** (JP2K decode/encode) — moved from v0.4 to v0.5+ on 2026-04-26. Filed as [#1](https://github.com/cornish/opentile-go/issues/1) with the full upstream algorithm, Go-side dependency tree, byte-parity bar, and trigger conditions for picking the work back up. Audit confirmed none of our 5 local SVS slides exhibit the corrupt-edge bug; 12 tasks of speculative cgo work for a synthetic-fixture-only feature didn't pass the project's "fix bugs we can demonstrate, don't write defensive code for hypothetical inputs" rule.

---

## 7. Retired in v0.5

Items closed during the v0.5 Philips TIFF milestone. One line per ID;
named commits' messages have the full rationale and the parity check
that locks the change in.

**Roadmap items (R-prefix):**

- **R5** — Philips TIFF support landed end-to-end (`1ad463c..7e7bde0`).
  New `formats/philips/` package, new `internal/jpegturbo.FillFrame`
  cgo entry point, new `internal/jpeg.InsertTables` (no-APP14 sibling
  to `InsertTablesAndAPP14`). All 4 sample fixtures
  (`Philips-{1,2,3,4}.tiff`) open cleanly: byte-identical to Python
  opentile 0.20.0 across every sampled tile and every associated
  image we expose. Parity oracle slate extended (11 slides total);
  integration suite covers all 12 fixtures (5 SVS + 3 NDPI + 4
  Philips) green on every commit.

**JIT verification gates (Tasks 1-3 of the v0.5 plan):**

- **T1** — `is_philips` detection gate (`f3ac48c`): all 4 Philips
  fixtures match upstream's `software[:10] == 'Philips DP'` AND
  `description[-16:].strip().endswith('</DataObject>')` rule; zero
  false positives across 13 non-Philips fixtures. No detection-rule
  refinement needed.
- **T2** — `FillFrame` determinism gate (`aa49f96`): Python's
  `Jpeg.fill_frame(src, 1.0)` is byte-deterministic across 5 passes
  (sha256 `05c3789cc691d9a207659e250b3fc9c799eca7c5019c4b084a441c4dca9da243`,
  2,364 bytes). Set the v0.5 sparse-tile blank-tile bar to byte
  equality. Cross-check during the FillFrame implementation (commit
  `e5ae3ac`) confirmed our Go `FillFrame` produces the SAME sha on
  the same input.
- **T3** — DICOM XML schema audit (`17cce32`): 11 DICOM_* tags
  inventoried across 4 fixtures. `DICOM_ACQUISITION_DATETIME` and
  `DICOM_DEVICE_SERIAL_NUMBER` absent on 3/4 fixtures (Philips-4
  only); multi-value strings (DICOM_SOFTWARE_VERSIONS,
  DICOM_LOSSY_IMAGE_COMPRESSION_*) are space-separated quoted. Drove
  the metadata parser's per-tag tolerant-of-absence design.

**Mid-task discoveries (where reading upstream changed the design):**

- The v0.5 plan's `computeCorrectedSizes` test expectation was based
  on a misread of `tifffile._philips_load_pages` (assumed N PS
  entries → N corrected sizes including baseline). Reading upstream
  byte-by-byte (the easily-missed `i += 1` at line 6540) corrected
  this: N PS entries → N-1 corrected sizes, with the first PS entry
  calibrating only.
- The synthesised-XML metadata tests passed under a flat
  `encoding/xml` schema, but the real Philips fixtures wrap
  level-specific Attributes inside `PIM_DP_SCANNED_IMAGES > Array >
  DataObject`. Smoke test against real fixtures forced a rewrite to
  a stack-based token decoder mirroring
  `ElementTree.iter('Attribute')`.
- `NativeTiledTiffImage.get_tile` always splices JPEGTables onto
  whatever `_read_frame` returns, including the cached blank tile
  (which already has tables inside `FillFrame`'s input). Result is
  duplicate DQT/DHT segments in the sparse-tile output — JPEG
  decoders accept this. Cross-check against Python at Philips-4 L0
  (0,0) caught our initial single-splice version.

---

## 8. Gate outcomes (live)

JIT verification gate outcomes from the v0.4, v0.5, and v0.6 plans.
Each gate decides a done-when bar or fix path for subsequent tasks.

### v0.6 gates

#### Task 5 — tifffile splice-replication harness

- **Date:** 2026-04-26
- **Outcome:** simpler than spec assumed. **OME files don't carry
  shared JPEGTables** — every page in our fixtures has
  `jpegtables=None`, every tile / strip's raw bytes start with
  SOI+JFIF (self-contained). For tiled levels, opentile-py's
  `get_tile()` output is **byte-identical to tifffile's raw bytes**
  with no splicing involved (verified on Leica-1 L0 (0,0):
  sha `668b391411f5ec95`, 17359 bytes, both paths).

- **Refined parity strategy** (revises spec §5):

  1. **Tiled levels of every Image**: tifffile reads raw tile bytes
     via `dataoffsets[idx]` / `databytecounts[idx]`; compare directly
     to our `Tile(x, y)` output. No Python-side splice needed. Works
     for all images including those opentile-py drops.

  2. **OneFrame levels of opentile-py-exposed images**
     (Leica-1 main; Leica-2 series 4): opentile-py's `get_tile()`
     gives the cropped output bytes. Compare directly to our output.

  3. **OneFrame levels of dropped images** (Leica-2 series 1-3): no
     byte-stable Python reference (opentile-py never sees them, and
     tifffile only gives raw single-strip bytes, not the cropped
     output). Coverage strategy:
     - Transitive correctness: our OneFrame implementation is the
       shared `internal/oneframe/` package validated by NDPI parity
       on every NDPI fixture.
     - Integration fixture SHAs: each OneFrame tile in those images
       gets a committed SHA snapshot; regressions are caught by
       `TestSlideParity`.

- **Consequence:** the new tifffile oracle (Task 19) is simpler than
  the spec's draft — no Python-side splice logic, just raw-byte read.
  ~30 LOC of Python rather than ~80.

#### Task 4 — OME-XML schema audit

- **Date:** 2026-04-26
- **Outcome:** clean. Both fixtures use the OME 2016-06 schema
  (namespace `http://www.openmicroscopy.org/Schemas/OME/2016-06`).
  Per-fixture Image inventory:

  | Fixture | Images | Names observed | PhysicalSize unit | Type |
  |---|---|---|---|---|
  | Leica-1.ome.tiff | 2 | `'macro'`, `''` (1) | µm on every Pixels | uint8 |
  | Leica-2.ome.tiff | 5 | `'macro'`, `''` × 4 | µm on every Pixels | uint8 |

  All Pixels elements carry PhysicalSizeX / PhysicalSizeY +
  PhysicalSizeXUnit / PhysicalSizeYUnit + SizeX / SizeY + Type
  attributes. No fixture is missing any. Empty Name attributes mean
  "main pyramid" per upstream's `_is_*_series` predicates (none of
  label / macro / thumbnail).

- **Consequence:** Task 12 (metadata parser) handles a single
  namespace URI (2016-06) for our fixtures. The parser should still
  use namespace wildcards (`xml:",any"` semantics) to be robust
  across OME schema versions — slides converted by older tooling
  may use earlier namespaces. All extracted fields (PhysicalSize,
  Size) can rely on µm units; reject other units as
  `ErrUnsupportedFormat` rather than half-implementing
  unit conversions. Type=uint8 is the only supported value (matches
  spec §8 out-of-scope).

#### Task 3 — OneFrame factor-or-copy decision

- **Date:** 2026-04-26
- **Outcome:** **factor**. Reading `formats/ndpi/oneframe.go` end-to-end:
  the body is already format-agnostic — single-strip JPEG read via
  generic `tiff.TagStripOffsets` / `tiff.TagStripByteCounts`, SOF
  parse and rewrite via `internal/jpeg`, MCU-aligned crop via
  `internal/jpegturbo`. No NDPI-specific tags (no McuStarts, no NDPI
  metadata refs). The only NDPI-shaped bits are the package name and
  one comment string.
- **Cross-check:** OME OneFrame levels also use single-strip
  JPEG-compressed pages whose strip bytes start with SOI+JFIF
  (self-contained — `jpegtables=None` on every OME fixture page).
  No JPEGTables splice required. Behaviour matches what
  `formats/ndpi/oneframe.go` already does.
- **Consequence:** Task 10 factors the body into `internal/oneframe/`.
  NDPI and OME both import it. Format-specific bits (MPP, pyramid
  index, level index) are set by the format package after
  construction. The factored package is a direct enabler for v0.7
  (BIF) which likely needs the same machinery.

#### Task 2 — SubIFD parsing audit

- **Date:** 2026-04-26
- **Outcome:** clean. SubIFDs (TIFF tag 330) are present on every
  OME pyramid base page and reachable via tifffile's `series.levels`
  API. Per-fixture inventory:

  | Fixture | Main series | SubIFDs / page | Total levels (top + sub) | Tiled / OneFrame split |
  |---|---|---|---|---|
  | Leica-1.ome.tiff | 1 (page 1 base) | 4 | 5 | L0/L1 tiled, L2/L3/L4 OneFrame |
  | Leica-2.ome.tiff | 4 (pages 1-4 bases) | 5 each | 6 each = 24 main pyramid levels | L0/L1 tiled, L2-L5 OneFrame |
  | (both) macro page | macro = page 0 | 2 | 3 (L0 used as AssociatedImage; L1/L2 ignored) | All OneFrame |

  Python opentile reports `5 levels` for Leica-1 main and `6 levels`
  for Leica-2's exposed main series — matching tifffile's `series.levels`
  one-for-one. Compression on every page is JPEG (7) on both fixtures.

- **Consequences:**
  1. `internal/tiff.Page.SubIFDOffsets()` (Task 6) and
     `tiff.File.PageAtOffset()` (Task 7) are required.
  2. **OneFrame levels are dominant** (3-4 of 5-6 levels per main
     pyramid). Skipping them in v0.6 would parity-skip the majority
     of pyramid output — Task 15 (OneFrame support) is mandatory,
     not optional.
  3. The macro's own pyramid (its 2 SubIFDs) is NOT exposed in our
     port; we use only macro L0 as the AssociatedImage, matching
     upstream.

#### Task 1 — `is_ome` detection gate

- **Date:** 2026-04-26
- **Outcome:** clean. Tifffile's detection rule
  (`page index == 0 AND description[-10:].strip().endswith('OME>')`,
  `tifffile.py:10125-10129`) matches both Leica OME fixtures and
  produces zero false-positives across the other 15 fixtures (5 SVS,
  3 NDPI, 4 Philips, 2 Ventana .bif, 1 generic TIFF). Description
  tails: `'</StructuredAnnotations></OME>'` on both Leica fixtures;
  every non-OME fixture's tail is unrelated. The rule is simpler
  than v0.5's spec draft assumed (which proposed a 3-clause check
  on `<?xml`, `<OME ` substring, and the OME namespace URL).
- **Consequence:** the v0.6 OME factory's `Supports()` predicate
  ports the rule verbatim — `strings.HasSuffix(strings.TrimSpace(desc[len-10:]), "OME>")`
  with bounds checks for short descriptions. No namespace-string
  matching needed.

### v0.5 gates

#### Task 1 — `is_philips` detection gate

- **Date:** 2026-04-26
- **Outcome:** clean. Tifffile's detection rule (`software[:10] == 'Philips DP'` AND `description[-16:].strip().endswith('</DataObject>')`) matches all 4 of our local Philips fixtures and produces zero false-positives across the other 13 fixtures (5 SVS, 3 NDPI, 2 OME, 2 Ventana .bif, 1 generic TIFF). Software values: `'Philips DP v1.0'` on every fixture; description tails: `'...</Attribute>\n</DataObject>'`. Non-Philips comparators that confirm the rule's specificity:
  - OME TIFF: `software='OME Bio-Formats 6.0.0-rc1'`, description tail `'</OME>'` — fails both clauses.
  - SVS: `software=None` — fails the prefix clause.
  - NDPI: `software='NDP.scan'` etc. — fails the prefix clause.
  - Ventana .bif: `software=None` or `'ScanOutputManager 1.1.0.15854'` — fails the prefix clause.
- **Consequence:** the v0.5 Philips factory's `Supports()` predicate ports the rule verbatim. No additional disambiguation needed.

#### Task 2 — `FillFrame` determinism gate

- **Date:** 2026-04-26
- **Outcome:** byte-deterministic. 5 passes through `Jpeg.fill_frame(src, 1.0)` in Python opentile (which wraps libjpeg-turbo's `tjTransform` with a CUSTOMFILTER that zeros all DCT coefficients, then sets the luma DC at the first block of each MCU) produced identical output bytes for the same source tile (SHA `05c3789cc691d9a207659e250b3fc9c799eca7c5019c4b084a441c4dca9da243`, 2,364 bytes; source: CMU-1-Small-Region.svs L0 tile (0,0), 3,985 bytes).
- **Consequence:** v0.5 sparse-tile blank-tile output (`internal/jpegturbo.FillFrame`) must byte-match Python opentile. Tasks 5 / 10 enforce byte-equality; the alternative pixel-equivalent fallback is not needed.

#### Task 3 — DICOM XML schema audit

- **Date:** 2026-04-26
- **Outcome:** the 11 attributes in upstream's TAGS list are all extractable from our 4 fixtures, but **two of them are missing on 3 of 4 fixtures** and three have multi-value formats. Concretely:
  - **Always present (4/4):** `DICOM_PIXEL_SPACING` (per-level series, count varies 9-11), `DICOM_BITS_ALLOCATED` (8), `DICOM_BITS_STORED` (8), `DICOM_HIGH_BIT` (7), `DICOM_LOSSY_IMAGE_COMPRESSION_METHOD`, `DICOM_LOSSY_IMAGE_COMPRESSION_RATIO`, `DICOM_MANUFACTURER`, `DICOM_PIXEL_REPRESENTATION` (0), `DICOM_SOFTWARE_VERSIONS`.
  - **Sometimes missing (1/4):** `DICOM_ACQUISITION_DATETIME` (only in Philips-4: `'20160718122300.000000'`), `DICOM_DEVICE_SERIAL_NUMBER` (only in Philips-4: `'FMT0107'`). Parser must return zero/empty for these on the other 3 fixtures.
  - **Multi-value strings** (space-separated, quoted): `DICOM_SOFTWARE_VERSIONS` (e.g. `'"1.6.6186" "20150402_R48" "4.0.3"'` on Philips-4), `DICOM_LOSSY_IMAGE_COMPRESSION_METHOD` (e.g. `'"PHILIPS_DP_1_0" "PHILIPS_TIFF_1_0"'`), `DICOM_LOSSY_IMAGE_COMPRESSION_RATIO`. Parser strips quotes and splits on whitespace.
  - **Scanner manufacturer is not always "Philips":** Philips-1 + Philips-3 say `Hamamatsu`, Philips-2 says `3D Histech`, Philips-4 says `PHILIPS`. The format is open — non-Philips scanners can emit Philips TIFF. Surfacing the actual manufacturer string verbatim is correct (no normalisation).
  - **`DICOM_PIXEL_SPACING` example value:** `'"0.000226891" "0.000226907"'` — quoted, space-separated `(W, H)` in metres-per-pixel. Floats with quotes around each. Parser strips quotes and splits.
  - **`DICOM_ACQUISITION_DATETIME` format:** `'20160718122300.000000'` — Go layout: `"20060102150405.000000"`. Microseconds are always zero in our one example.
- **Consequence:** Task 11 (PhilipsTiffMetadata XML parser) handles missing values via Optional / pointer-to-T patterns, splits on whitespace + strips `"` for the multi-value string fields, parses dates with the documented Go layout. No additional disambiguation needed beyond what upstream's parser already encodes.

### v0.4 gates

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

## 9. Triage process

Once the branch lands on a remote, every numbered item above should become a tracked issue (GitHub, Linear, etc.) — scope items → roadmap epics, limitations → user-facing docs, reviewer suggestions → individual backlog tickets. Delete entries from this file as they get filed. The goal is for this file to eventually shrink to zero as polish milestones retire each item.
