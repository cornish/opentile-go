# opentile-go v0.3 design

**Status:** draft, brainstorming-approved 2026-04-24.
**Predecessor:** [v0.2 design](./2026-04-21-opentile-go-v02-design.md), shipped as `v0.2.0` on `main`.

## Goal

Close every known v0.2 limitation, reviewer suggestion, and review nice-to-have. v0.3 is the **settled API + test-complete + correctness-thorough** milestone before v0.4+ takes on new format support. No new formats land in v0.3.

## Architecture

Single polish milestone, no new external dependencies, no new packages. The existing v0.2 architecture (`internal/tiff`, `internal/jpeg`, `internal/jpegturbo`, `formats/{svs,ndpi,all}`, `tests/oracle`) stays. Work is organized into nine themes whose changes are mostly local refactors, additional tests, doc/comment additions, and four meaningful production-code fixes (L1, L7, L10, L11).

## Tech stack

Same as v0.2: Go 1.23+, libjpeg-turbo 2.1+ via cgo (`internal/jpegturbo`), Python opentile 0.20.0 via subprocess (parity oracle, opt-in via `//go:build parity`). No new dependencies in `go.mod` or `tests/oracle/requirements.txt`.

---

## Theme 1 — API settlement

The "settled API" claim hangs on these. v0.3 freezes the public surface: every name in `go doc ./...` after v0.3 ships survives v0.3 → v0.4 unchanged unless explicitly versioned.

### T1 — `NewTestConfig` moves to `opentile/opentiletest`

Move `opentile.NewTestConfig` from `options.go` into a new sibling package `opentile/opentiletest`:

```go
package opentiletest

func NewConfig(tileSize opentile.Size, policy opentile.CorruptTilePolicy) *opentile.Config { ... }
```

Existing callers (`formats/ndpi/bench_test.go`, `formats/svs/svs_test.go`, etc.) are updated. The old name stays for one release as a deprecation alias:

```go
// Deprecated: use opentile/opentiletest.NewConfig.
func NewTestConfig(tileSize Size, policy CorruptTilePolicy) *Config { ... }
```

### A1 — `ErrTooManyIFDs` sentinel

Promote `internal/tiff.walkIFDs`'s safety-cap error from a formatted string to a public sentinel:

```go
package opentile
var ErrTooManyIFDs = errors.New("opentile: TIFF IFD chain exceeded the safety cap")
```

Callers can `errors.Is(err, opentile.ErrTooManyIFDs)`. Same treatment for any analogous per-IFD entry-count cap if `walkIFDs` has one.

### A2 — `OpenFile` includes path in error

```go
return nil, fmt.Errorf("opentile: open %s: %w", path, err)
```

### A3 — `Formats() []Format` introspection helper

```go
// Formats returns the format identifiers that have been registered via
// Register, sorted lexicographically. Useful for diagnostics and for callers
// that want to enumerate what builds in without importing each format
// package directly.
func Formats() []Format { ... }
```

### A4 — `Config.TileSize() (Size, true)` semantics for explicit `Size{0,0}`

Document in godoc that `(Size{}, true)` means "caller explicitly passed zero" and format packages MUST reject as malformed input (versus `(Size{}, false)` which means "use format default"). No code change; one new test in `options_test.go` confirming the round-trip.

### N-5 — `WithNDPISynthesizedLabel(bool)` option

NDPI's synthesized label (cropping the overview) is a Go-side extension (L14). Add an opt-out:

```go
opentile.OpenFile(path, opentile.WithNDPISynthesizedLabel(false))
```

Default stays `true` (matches v0.2 behavior). When `false`, NDPI `Tiler.Associated()` excludes the label kind. L14 in `docs/deferred.md` updates from "deliberate divergence" to "deliberate divergence, opt-out via `WithNDPISynthesizedLabel(false)`."

### N-7 — `%w` wrapping consistency in `formats/ndpi`

Audit every error path in `formats/ndpi/{oneframe.go,striped.go,stripes.go,associated.go,ndpi.go}`. Ensure every error that could plausibly carry an underlying `io.EOF` / `tiff.Err...` / `jpegturbo.Err...` uses `fmt.Errorf("...: %w", err)` so callers can `errors.Is`. No new error types — just consistency.

**Migration impact:** the only true breaking change is T1 (call sites move). Everything else is additive (new sentinels, helpers, options) or doc-only.

---

## Theme 2 — Test coverage push

Goal: every package ≥80% statement coverage with `OPENTILE_TESTDIR` set, public API surface fully exercised. Enforced by a `make cover` target as the v0.3 done-when gate.

### Trivial-getter coverage

The 15 zero-coverage getters (`Index`, `PyramidIndex`, `MPP`, `FocalPlane`, `Format`, `ICCProfile`, `Photometric`, `SamplesPerPixel`, `BitsPerSample`) get exercised by extending `TestSlideParity` (in `tests/integration_test.go`) to assert each Level's geometry/metadata fields against the fixture. Most fields are already in fixtures; the test just needs to read them.

### Public-API coverage (real gap)

Add `TestTileReaderRoundTrip` to `formats/svs` and `formats/ndpi` test packages: for each level, pick one tile, read via `Tile()` AND `TileReader()`, assert `bytes.Equal`. Add `TestTilesIterRowMajor`: walk `Tiles(ctx)`, confirm row-major ordering and that every result matches `Tile(x, y)`.

### Branch coverage (specific tests)

- **T3** — `internal/tiff/ifd_test.go`: synthetic two-IFD builder where IFD 1's "next" pointer points back at IFD 0; assert `errors.Is(err, opentile.ErrTooManyIFDs)` and the cycle is rejected within `maxIFDs` iterations.
- **T4** — `formats/svs/tiled_test.go`: synthetic TIFF page with `Compression=999` → expect `CompressionUnknown`.
- **T5** — `formats/svs/metadata_test.go`: parse `ImageDescription` strings with CRLF, LF-only, and CR-only line endings; with leading/trailing whitespace; with duplicate keys (later wins). Locks the L1 regression.
- **T6** — `internal/tiff/ifd_test.go`: rewrite `TestWalkIFDsMultiple` to actually build a 3-IFD chain and verify every IFD gets walked (not just single-IFD termination).

### Format-specific tests

- **L3** — `formats/svs/tiled_test.go`: synthetic TIFF page with `Compression=5` (LZW) on the tiled levels path; confirms `mapCompression` returns `CompressionLZW` AND that `Tile()` returns raw bytes (passthrough). No real LZW slide needed.
- **L9** — `internal/jpegturbo/turbo_cgo_test.go`: spawn 32 goroutines each running 200 crops on a small fixture JPEG, assert all outputs byte-identical to the single-threaded result. Locks the safe-for-concurrent-use contract.

### Coverage gate

Add `make cover` (or `scripts/cover.sh`):

```sh
OPENTILE_TESTDIR="$PWD/sample_files" go test ./... -coverpkg=./... -coverprofile=/tmp/cov
go tool cover -func=/tmp/cov | awk '... per-package ≥80% check ...'
```

Exits non-zero on any package below 80%. Becomes the v0.3 done-when gate.

### Non-goals

We don't add tests for trivially uncoverable paths (cgo error handling that requires fault-injecting libjpeg-turbo, OS-level allocation failures). If a function is at 60% because the remaining 40% is a malloc-failure branch, leave it.

---

## Theme 3 — Format-correctness fixes

Five items; the only theme that meaningfully changes production behavior.

### L1 — `SoftwareLine` trailing `\r`

One-line fix in `formats/svs/metadata.go`:

```go
md.SoftwareLine = strings.TrimRight(desc[:newline], "\r\n ")
```

T5's CRLF parser tests lock the regression. Existing CMU-1 fixture metadata changes; regenerate.

### L7 + L11 — actual MCU detection from SOF

Both items hardcode `mcu = 16×16`. Single shared helper in `internal/jpeg`:

```go
// MCUSizeOf reads the SOF0 segment of jpeg and returns the MCU pixel size,
// derived from each component's sampling factors. For YCbCr 4:2:0 this is
// 16×16; for 4:2:2 it's 16×8; for 4:4:4 (or grayscale) it's 8×8.
func MCUSizeOf(jpeg []byte) (w, h int, err error)
```

Implementation is ~20 lines using `Scan` + `ParseSOF` + the existing sampling-factor math from `ndpi_tile.go:NDPIStripeJPEGHeader`. Then:

- `formats/ndpi/ndpi.go` reads the overview's first JPEG bytes once, calls `MCUSizeOf`, passes the result to `newLabelImage` instead of hardcoded `(16, 16)`.
- `formats/svs/associated.go:computeRestartInterval` does the same per-page (cached at constructor time, not per-tile).

Acceptance: fixture-backed tests pass on existing slides (no real change for 4:2:0 inputs); add a synthetic 4:2:2 fixture to verify the non-default path. Retire L7 and L11.

### L10 — SVS LZW label decode-restitch-encode

Currently the SVS label returns strip-0 bytes only (a sliver), matching upstream Python opentile's bug. v0.3 fixes Go-side:

1. **Decode path:** for each strip, decode via Go stdlib `compress/lzw.NewReader(r, lzw.MSB, 8)`. TIFF LZW uses MSB-first bit ordering with 9-bit codes ramping up; `compress/lzw` with `lzw.MSB` and `litWidth=8` matches.
2. **Restitch:** concatenate the decoded raster (each strip is `RowsPerStrip` × `ImageWidth` × samples bytes). For partial last strip (`ImageLength % RowsPerStrip != 0`), trim.
3. **Re-encode:** `compress/lzw.NewWriter(w, lzw.MSB, 8)` over the full raster. Output is a single LZW stream covering the full image height.
4. **Update fields:** `Size()` now reports full image size (currently lies). `RowsPerStrip` not exposed externally so no caller-facing change. Compression stays `CompressionLZW`.

**Parity strategy:** Python opentile still returns strip 0 only. After the fix, byte parity for SVS labels diverges. Cleanest v0.3 path:
- Gate the parity oracle on `kind == "label"` (skip), same pattern we already use for NDPI labels.
- File an upstream PR against `imi-bigpicture/opentile` proposing the same restitch for Python; record the PR link in `docs/deferred.md` so future contributors know parity becomes byte-equal once upstream merges.

A unit test decodes a synthetic 3-strip LZW raster, restitches, re-encodes, and verifies the round-trip equals concatenating the strip-decoded rasters.

Retire L10.

### BigTIFF SVS integration (when slide arrives)

Mechanical:
1. Drop slide into `sample_files/svs/`.
2. Add filename to `slideCandidates` in `tests/integration_test.go`.
3. `OPENTILE_TESTDIR=$PWD/sample_files go test ./tests -tags generate -run TestGenerateFixtures -generate -v` to produce the fixture.
4. Run parity oracle — surface any divergence.
5. If parity holds, commit as a fixture addition. If new bugs surface (BigTIFF + SVS combination may hit untested paths in `internal/tiff`), file them as discovered tasks within v0.3.

Lands whenever the download completes — doesn't gate the rest of the milestone. If still pending at end of v0.3, deferred to v0.3.1 with a deferred entry.

### Retirements (deferred.md cleanup only)

- **L2** ("Non-tiled pages silently skipped") — addressed in v0.2 by associated-image work. Move to v0.2 history section.
- **L8** ("SVS v0.1 page classifier was guessed") — verified during Task 21 in v0.2. Retire.
- **L13** ("v0.2 NDPI striped path was architecturally wrong") — historical process note; keep in §4 v0.2 session learnings, remove from active limitations.

---

## Theme 4 — Performance + DRY

Five items; mostly local refactors.

### N-1 — unify `patchSOFSize` and `ReplaceSOFDimensions`

Two near-identical SOF-dimension patchers exist (`formats/ndpi/striped.go:patchSOFSize` and `internal/jpeg/sof.go:ReplaceSOFDimensions`). Merge into one canonical helper in `internal/jpeg`, delete the duplicate, update both callers. Net negative LOC.

### N-3 — memoize `LuminanceToDCCoefficient`

Currently re-computed on every `CropWithBackgroundLuminance` call inside `internal/jpegturbo/turbo_cgo.go`. For an NDPI level with thousands of edge tiles this re-parses the same DQT N times. Add a `BackgroundLuminanceDC int` field to `Region` (or a new `CropOpts`) and let format packages pass the precomputed DC value, dropping the parse from the cgo path entirely. Format packages compute the DC value once at level-construction time.

### I3 — `Scan` bufio chaining

`internal/jpeg.Scan` wraps its `io.Reader` unconditionally in `bufio.NewReader`. `ReadScan` does the same dance. Refactor `Scan` to detect an existing `*bufio.Reader` and reuse it (the pattern `ReadScan` already uses), letting callers chain `Scan` → `ReadScan` on a single underlying reader. `extractScanData`'s byte-scan workaround in `internal/jpeg/concat.go` then becomes a real `Scan` call.

### I4 — `ConcatenateScans` precomputed header prefix

Currently re-runs `SplitJPEGTables` + first-fragment SOF/SOS extraction per call. For 24K-tile slide levels this reparses the same JPEGTables blob 24K times. Replace with an `Assembler` struct (or per-level `sync.Once`-cached header prefix) that holds the precomputed bytes; per-call work becomes `header + fragments + tail`. Profile-driven; expect a measurable per-tile improvement on multi-strip SVS associated images. Doesn't change byte output.

### O1 — `walkIFDs` bulk-read

Currently 4 `ReadAt` calls per entry. Replace with a single `ReadAt` for the full IFD (`2 + 12*count + 4` bytes), then slice-decode in memory. Helps adversarial inputs with inflated entry counts and reduces syscall pressure on real slides.

---

## Theme 5 — Robustness

Five items; defense-in-depth without changing byte output for any current real fixture.

### N-2 — segment-walker for SOS/DRI lookup

`internal/jpeg.ConcatenateScans` uses `bytes.Index(frame, {0xFF, 0xDA})` to locate SOS. Doc-comment concedes the assumption that byte-stuffing prevents `0xFF` in entropy data. Replace with `Scan`-based marker walking — finds the *real* SOS, never a false-positive in a quant table or comment payload. Same treatment for any other byte-scan SOS/DRI/SOF lookups in the package.

### N-4 — DQT byte-scan validates segment boundaries

`internal/jpeg/dqt.go:dcQuantForTable` byte-scans for `FF DB`. After finding it, validate the offset matches a real segment boundary (i.e. the prior segment's length brings us to exactly this byte) before trusting it. Falls back to byte-scan only if the structural walk fails. Cheap insurance against pathological inputs.

### N-10 — invert striped fall-through control flow

`formats/ndpi/striped.go` currently tries `Crop` first; on any error, checks `extendsBeyond` and falls through to `CropWithBackground`. An unrelated `Crop` failure (corrupt entropy stream) would still try the OOB fill and probably succeed-but-render-wrong. Invert: check `extendsBeyond` first; if yes, call `CropWithBackground` directly; else `Crop` only. Mirrors upstream control flow.

### I5 — defensive RSTn count check in `ConcatenateScans`

`ConcatOpts.RestartInterval > 0` assumes each fragment contains exactly one restart interval (no internal RSTn markers). Documented today, not enforced. Add a defensive scan that counts RSTn markers in each fragment; error on mismatch. Doesn't fire for our NDPI/SVS inputs but catches future formats that violate the assumption.

### O2 — `int(e.Count)` overflow comment

`internal/tiff/page.go` does `int(e.Count)` for `JPEGTables` / `ICCProfile` reads. Theoretical truncation on 32-bit platforms with `Count > 2 GiB`. Add a one-line comment that this is bounded by real-world tag values (<1 MB) on 64-bit targets where we run; document the assumption rather than introduce a defensive cap nobody can hit.

---

## Theme 6 — Documentation

Eight in-code items, each 1-3 lines. Single commit.

- **D1** — `internal/tiff/decodeASCII`: note that missing NUL terminator is silently tolerated.
- **D2** — `internal/tiff/decodeInline`: explain why the function takes a full `*byteReader` when it only uses `b.order`.
- **D3** — `Metadata.AcquisitionDateTime` godoc: partial Date/Time input yields zero-value time; zero is the "unknown" sentinel.
- **N-6** — `jpegturbo.CropWithBackground` godoc: chroma DC=0 produces neutral chroma, so OOB output is *near* but not *exact* white. Calls out the visual.
- **N-9** — `internal/tiff/file.go`'s NDPI tag-65420 sniff: existing L5 entry already documents the architectural call; add a corresponding source comment pointing at L5.
- **I2** — `internal/tiff/ifd.go:walkIFDs`: note that overlapping IFDs (one IFD's body containing another IFD's start) are not detected; only exact-offset cycles via the seen-offset map.
- **I6** — `internal/jpeg/marker.go:isStandalone`: switch literal `0xD0..0xD7` to ranged check `m >= RST0 && m <= RST0+7`. Code-and-comment change.
- **I7** — `internal/jpeg.ReplaceSOFDimensions` (after N-1 unification, this is the canonical helper): comment explaining the SOF-precedes-SOS-in-well-formed-JPEGs invariant that makes byte-scan safe.

---

## Theme 7 — Parity oracle batched runner (L16)

Single change. `tests/oracle/oracle_runner.py` currently spawns once per tile. Replace with a stdin-driven runner:

```
oracle_runner.py <slide>
# stdin: one position per line, "level x y" or "associated kind"
# stdout: length-prefixed (4-byte big-endian uint32) blobs back, in order
```

`tests/oracle/oracle.go` opens one persistent subprocess per slide, writes positions, reads blobs. Net effect: ~200 ms Python startup paid once per slide instead of once per tile.

`samplePositions` default raised from 10 → 100 per level. `-parity-full` still walks every tile but is now O(1) Python startups too. Default-mode runtime stays under a minute per slide for the 100-tile sample; `-parity-full` runtime drops by ~10× on OS-2.ndpi.

L12 stays punted to v0.4+ — we don't add white-fill investigation in v0.3.

---

## Theme 8 — Refactors

Three items.

### T2 — `resetRegistry` parallelism

`opentile_test.go` uses a package-global registry with `resetRegistry()` in each test. Unsafe if any test ever calls `t.Parallel()`. Replace with a helper:

```go
func withRegistry(t *testing.T, factories ...FormatFactory) {
    t.Helper()
    saved := registry
    t.Cleanup(func() { registry = saved })
    registry = nil
    for _, f := range factories { Register(f) }
}
```

Tests opt into specific factories per-test instead of mutating global state.

### I1 — fold `length == 0` check into `indexOf`

`formats/svs/tiled.go:Tile` and `TileReader` each duplicate the zero-length corrupt-tile check. Move into `indexOf` so both call sites become single-line.

### I8 — `paddedJPEGOnce` → `sync.Once`

`formats/ndpi/oneframe.go`: replace the plain bool with `sync.Once`. The current code is safe in practice (per the I-2 comment we updated in v0.2), but `sync.Once` makes the contract explicit and prevents a future caller adding a non-`extendedOnce.Do` entry from regressing.

---

## Theme 9 — Hamamatsu-1.ndpi sparse fixture (L15)

The 6.4 GB Hamamatsu-1.ndpi exercises the NDPI 64-bit offset extension. Its full fixture (every tile SHA hashed) would exceed our 5 MB soft cap. v0.2 punted; the 64-bit-offset code path has zero integration-test coverage.

### Schema extension

Extend `Fixture` with an optional sampled-tile map alongside the existing full map:

```go
type Fixture struct {
    Slide             string
    Format            string
    Levels            []LevelFixture
    Metadata          MetadataFixture
    TileSHA256        map[string]string         `json:"tiles,omitempty"`         // full
    SampledTileSHA256 map[string]SampledTile    `json:"sampled_tiles,omitempty"` // optional
    ICCProfileSHA256  string                    `json:"icc_profile_sha256,omitempty"`
    AssociatedImages  []AssociatedFixture       `json:"associated,omitempty"`
}

type SampledTile struct {
    SHA256 string `json:"sha256"`
    Reason string `json:"reason"` // human-readable description of what code path the tile covers
}
```

### Two-layer sampled-position rule

The generator picks one mode per slide based on a `-sampled` flag (or auto-detects: if estimated full size > 5 MB, fall back to sampled). Sampled mode picks positions in two layers:

**Layer 1 — position-based (existing `samplePositions`):**
- Four corners, three diagonal interior points, three near-edge mid-row/col points.

**Layer 2 — circumstance-based (computed from level geometry):**
- Bottom-edge tile in middle row (`(W/2, H-1)`): exercises the "below image" white-fill callback when `image_h % tile_h != 0`.
- Right-edge tile in middle column (`(W-1, H/2)`): exercises the "right of image" callback.
- Bottom-right corner crossing both axes (already in Layer 1, retained).
- First tile of the last native stripe row (NDPI): catches `McuStarts` indexing bugs at the tail.
- Tile whose frame is sub-MCU-aligned (when `image % MCU != 0`): exercises SOF-padding + OOB callback interaction.

Each Layer-2 position carries a short `reason` label in the fixture JSON, so when a fixture diff lands and one of these flips, the reviewer immediately knows what code path is affected.

### Integration test

`TestSlideParity` checks whichever field is populated:
- `TileSHA256` non-empty → walk every tile, hash, compare. (Existing behavior; SVS + CMU-NDPI + OS-NDPI fixtures use this.)
- `SampledTileSHA256` non-empty → walk only the sampled positions, hash, compare. (New for Hamamatsu-1.)
- Both → walk both; intersection must match.

### Outcome

1. Add `Hamamatsu-1.ndpi` to `slideCandidates`.
2. Generate sampled fixture (~100-200 KB JSON).
3. Commit. CI / `OPENTILE_TESTDIR` integration runs now exercise the 64-bit offset path on every commit.
4. Parity oracle runs against Hamamatsu-1 with the same sampled positions.

L15 retires once the Hamamatsu-1 sampled fixture lands. The pattern generalizes to v0.4+ formats (Philips 4.5 GB, DICOM-WSI 541 MB) — they default to sampled mode; SVS/CMU-NDPI/OS-NDPI keep full fixtures.

---

## Implementation order

Roughly 25-30 commits across five execution batches:

- **Batch A** — Theme 1 (API settlement) + Theme 9 (Hamamatsu-1 sparse fixture). API surface settles first because subsequent tests reference the new helpers; Hamamatsu-1 lands early so the 64-bit-offset path is in CI before any subsequent change can regress it silently.
- **Batch B** — Theme 3 (correctness fixes). L1, L7, L11 land. L10's LZW restitch is its own commit. BigTIFF SVS slot — lands whenever the slide arrives.
- **Batch C** — Theme 2 (test coverage push). After Theme 3 so new tests cover corrected code. Coverage gate (≥80% per package) becomes the v0.3 done-when.
- **Batch D** — Themes 4, 5, 7 (perf + robustness + batched parity oracle).
- **Batch E** — Themes 6, 8 + final review + retirement of L1/L2/L7/L8/L10/L11/L13/L14/L15/L16 from active list. BigTIFF SVS integration commit if slide has arrived; otherwise deferred to v0.3.1.

## Acceptance criteria

`feat/v0.3` ships when:

- All 16 L-numbers either retired or actively documented as permanent (L4, L5, L6, L13, L14 stay as "permanent" or "process note"; L1/L2/L3/L7/L8/L9/L10/L11/L15/L16 retire; L12 stays open with a clear v0.4+ pointer).
- All N-1 through N-10 from the v0.2 final review either landed or explicitly retired with a deferred entry.
- All T/A/O/D/I reviewer suggestions either landed or retired (T1, T2, T3, T4, T5, T6; A1, A2, A3, A4; O1, O2; D1, D2, D3; I1, I2, I3, I4, I5, I6, I7, I8 — all addressed).
- Coverage ≥80% per package with `OPENTILE_TESTDIR` set, enforced by `make cover` (or equivalent script).
- `TileReader` and `Tiles` exercised in both formats.
- Hamamatsu-1.ndpi sparse fixture committed and integration test green.
- BigTIFF SVS fixture committed *if* slide is available; otherwise BigTIFF-SVS integration deferred to v0.3.1 with a deferred entry.
- Parity oracle runtime under 60s for the sampled run on all 5+ slides.
- `go test ./... -race -count=1` clean. `go vet ./...` clean. `CGO_ENABLED=0 go test ./... -tags nocgo` clean.
- Public API stable from this point: every name in `go doc ./...` survives v0.3 → v0.4 unchanged unless explicitly versioned.

## Out of scope (deferred to v0.4+)

- All new format support (Philips, DICOM-WSI, MRXS, OME-TIFF, Ventana BIF, generic TIFF beyond what `internal/tiff` already handles).
- L12 (NDPI edge-tile entropy nondeterminism) — explicit punt with a clearer investigation framing for v0.4+.
- JPEG 2000 decode (R9) — only matters once associated-image re-encoding lands.
- L10 upstream PR to Python opentile — file separately, not a v0.3 deliverable.

## Cross-cutting notes

- The `feedback_ndpi_architecture.md` cross-session memory stays as-is; no v0.3 update needed.
- `CLAUDE.md` gets the milestone bump (v0.2 → v0.3) and the "API stable from this point" invariant added.
- `README.md`'s "byte-identical parity" claim stays unchanged — Theme 7's batched runner doesn't change parity coverage, just speed.
- After v0.3 ships and the deferred-list is shorter, that file becomes the primary input to v0.4 brainstorming.
