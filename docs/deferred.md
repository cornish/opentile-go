# Deferred Issues

Running log of items that are intentionally out of scope for v0.1 or were raised during code review and parked for later. Once this branch lands on a hosting service, triage everything below into real tracked issues and delete the entries as they are filed.

Format per item:
- **ID** — short descriptor
- **Source** — commit or review where the item originated
- **Severity** — Scope / Limitation / Suggestion
- **Disposition** — target milestone or backlog note

---

## 1. Roadmap items (scope-deferred by design)

These were excluded from v0.1 in the brainstorming / design phase. They are not bugs; the library intentionally does not address them yet.

| ID | Feature | Target |
|----|---------|--------|
| R1 | NDPI format support (Hamamatsu) | v0.2 |
| R2 | `internal/jpeg` marker package (needed for NDPI stripe concatenation) | v0.2 |
| R3 | SVS associated images — label, overview, thumbnail | v0.3 |
| R4 | Aperio SVS corrupt-edge reconstruct fix (currently returns `ErrCorruptTile`) | v1.0 |
| R5 | Philips TIFF (sparse-tile filler) | v0.4 |
| R6 | 3DHistech TIFF | v0.5 |
| R7 | OME TIFF | v0.5 |
| R8 | BigTIFF support (currently rejected as `ErrUnsupportedTIFF`) | when a format needs it |
| R9 | JPEG 2000 decode/encode (native tiles pass through today; decoding is needed only if associated-image re-encoding lands) | v0.3+ |
| R10 | Remote I/O backends (S3, HTTP range, fsspec equivalents) | out-of-scope; consumers supply their own `io.ReaderAt` |
| R11 | Python parity oracle under `//go:build parity` | v0.2 (lands with NDPI) |
| R12 | CLI wrapper | out-of-scope for v1 |

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

---

## 4. Process note

Once `feat/v0.1` lands on a remote, every item above should be triaged into a tracked issue (GitHub, Linear, etc.) — scope items become roadmap epics, limitations become user-facing docs, reviewer suggestions become individual backlog tickets. Delete entries from this file as they get filed. The goal is for this file to eventually shrink to zero as v0.1 polish, v0.2, and ongoing maintenance retire each item.
