# opentile-go

Direct Go port of [imi-bigpicture/opentile](https://github.com/imi-bigpicture/opentile) (Apache 2.0, Sectra AB) with one cgo dependency (libjpeg-turbo, narrowly scoped to `internal/jpegturbo/`). Reads tiles from WSI (whole-slide imaging) TIFF files used in digital pathology.

## Current milestone — v0.7

- **Scope:** Ventana BIF (Roche / iScan) support — the fifth format opentile-go handles and **the first beyond upstream Python opentile's coverage**. New `formats/bif/` package; new `internal/bifxml/` XML walker; new `Level.TileOverlap() image.Point` interface method (additive evolution); new `formats/bif/blanktile.go` empty-tile generator; new tests/parity/ package + bif_geometry_test; openslide + tifffile parity oracle infrastructure under `tests/oracle/`. Two real fixtures (Ventana-1 spec-compliant DP 200 + OS-1 legacy iScan Coreo) round-trip through `opentile.OpenFile` cleanly. **Mid-v0.7 multi-dim closeout** added cross-format multi-dim addressing — `TileCoord` + `Level.TileAt` + `Image.SizeZ/SizeC/SizeT/ChannelName/ZPlaneFocus` — and retired L21 (BIF reads `IMAGE_DEPTH` Z-stacks natively; OME surfaces honest dimensions).
- **API extension:** `Level.TileOverlap() image.Point` returns the per-tile-step pixel overlap; non-zero only on BIF level 0 (the only level with `<TileJointInfo>` overlap entries per spec). Existing format levels return `image.Point{}` — no caller change required. **Multi-dim addition (mid-v0.7):** `Level.TileAt(TileCoord{X, Y, Z, C, T})` plus `Image.SizeZ/SizeC/SizeT/ChannelName/ZPlaneFocus`; 2D formats inherit `SingleImage` defaults so `Tile(x, y) == TileAt(TileCoord{X: x, Y: y})` byte-identically. New `ErrDimensionUnavailable` sentinel discriminates "axis absent" from "axis index past size" (`ErrTileOutOfBounds`).
- **Active limitations:** L4, L5, L14 (Permanent — carried over from v0.6) plus two v0.7 work items deferred to v0.8+ (`docs/deferred.md §2`): L19 (openslide pixel-equivalence on BIF — coordinate-system gap, infrastructure-only ships in v0.7), L20 (DP 600 unverified — fixture-dependent). L21 (Volumetric Z-stacks) was retired by the v0.7 multi-dim closeout.
- **Deviations from upstream Python opentile** (canonical list at `docs/deferred.md §1a`): NDPI synthesised label (v0.2), NDPI Map page surfacing (v0.4), multi-image OME pyramid exposure (v0.6), OME PlanarConfiguration=2 plane-0-only indexing (v0.6), OME first-strip-only on multi-strip OneFrame (v0.6), BIF probability map exposure (v0.7), BIF `Level.TileOverlap()` (v0.7), BIF non-strict `ScannerModel` acceptance (v0.7), multi-dim WSI API addition (v0.7 mid-milestone closeout).
- **Correctness bar revision:** the v0.7 design spec §7 originally framed openslide pixel-equivalence as the primary BIF oracle. Mid-implementation we found openslide rejects spec-compliant DP 200 BIFs (`Direction="LEFT"`) and uses an AOI-hull coordinate system that doesn't match opentile-go's padded TIFF view. Anecdotal community note: openslide is also believed to misread modern BIF generally. v0.7's actual correctness bar is **tifffile byte-equality on Ventana-1** + **committed sample-tile SHA256 hashes via `TestSlideParity`** for both fixtures. openslide-pixel-equivalence is a v0.8 follow-up (L19).
- **Deferred:** R4 (SVS corrupt-edge reconstruct) + R9 (JP2K decode/encode) parked at [#1](https://github.com/cornish/opentile-go/issues/1). R6 (3DHistech TIFF) parked at [#2](https://github.com/cornish/opentile-go/issues/2); R15 (Sakura SVSlide) parked at [#3](https://github.com/cornish/opentile-go/issues/3). v0.8 will likely tackle L19 + L20 + Leica SCN (R16) — TBD based on real-slide demand.
- **Design:** `docs/superpowers/specs/2026-04-27-opentile-go-v07-design.md`
- **Plan:** `docs/superpowers/plans/2026-04-27-opentile-go-v07.md`
- **Research notes:** `docs/superpowers/notes/2026-04-27-bif-research.md`
- **Work branch:** `feat/v0.7`

## Invariants

- **Public API stable from v0.3.** Adding new exported names is fine; renaming, moving, or removing is a breaking change that requires a major-version bump (or, until we have external users, an explicit owner sign-off).
- **Don't guess format behavior — read upstream.** This is a **direct port** of Python opentile (which delegates format details to tifffile). Whenever classification, layout, tag semantics, or edge-case handling is unclear: **read `imi-bigpicture/opentile` first, then `cgohlke/tifffile`**. Guessed behavior cost v0.2 five separate debugging cycles (NDPI IFD layout, NDPI metadata tag numbers, NDPI StripOffsets tag, NDPI striped vs. oneframe gate, APP14 byte values) — every one fixed by reading the actual upstream source. The v0.4 plan elevates this to a structural per-task `Step 0: Confirm upstream` action that the executor must run before any production-code edit. The rule: if you catch yourself reasoning from first principles about a WSI format quirk, stop and find the upstream code that handles it. Port directly, adapt for Go idioms, but preserve the logic.
- **No cutting corners; no active users yet.** Complete things we know are broken before moving on. When a bug is identified, the rule is: fix it, don't defer. Plan thoroughly for v0.3+ rather than race.
- **Architectural placement of ported logic:** format-specific quirks belong in the format package (`formats/ndpi/`, `formats/svs/`), not `internal/tiff`. `internal/tiff` stays a generic TIFF/BigTIFF/NDPI-IFD parser. Examples: NDPI page-series grouping, SVS ImageDescription quirks, Philips sparse-tile filling.
- **cgo is narrowly scoped.** `internal/jpegturbo/` is the only package linking libjpeg-turbo. Under `nocgo` build tag, format paths that need it return `ErrCGORequired`; the rest works.
- **Direct port under Apache 2.0** with attribution retained in `NOTICE`. Not affiliated with or endorsed by Sectra AB or the BigPicture project.
- **Parity with upstream is the correctness bar.** Upstream's pytest cases are ported to Go tests; a fixture-backed integration suite compares tile bytes against a committed snapshot. An opt-in `//go:build parity` harness that shells out to Python opentile is v0.2.
- **Lock-free hot path for metadata.** Parsed IFDs, per-tile offset/length arrays, and metadata are populated at `Open()` time and immutable thereafter. `Tile()` is safe to call concurrently from many goroutines — the shared-state caches in `formats/ndpi/striped.go` (per-frame assembly cache) and `formats/ndpi/oneframe.go` (extended-frame cache) use double-checked locking and `sync.Once` respectively and produce byte-deterministic results regardless of which goroutine populates them first.

## Conventions

- Module path: `github.com/cornish/opentile-go`
- Go 1.23+ (for `iter.Seq2`)
- `internal/tiff` and `internal/jpeg` are internal — both shaped for opentile's needs, not general-purpose libraries. `internal/jpegturbo` is the only cgo package in the module.
- Format subpackages (`formats/svs/`, `formats/ndpi/`, …) are public; `formats/all` is the umbrella registration package
- `io.ReaderAt` + `int64` size is the core input (stdlib `*os.File` satisfies concurrent-use semantics)
- Public tile methods: `Level.Tile(x, y int)` returns raw compressed bytes; `Level.TileReader(x, y)` streams via `io.SectionReader`; `Level.Tiles(ctx)` is serial row-major via `iter.Seq2`

## Sample slides

Local slides live in `/sample_files/` (gitignored). v0.6 fixture set:
- `sample_files/svs/CMU-1-Small-Region.svs` (1.9 MB, JPEG) — primary fixture
- `sample_files/svs/CMU-1.svs` (177 MB, JPEG) — full-slide fixture
- `sample_files/svs/JP2K-33003-1.svs` (63 MB, JPEG 2000 passthrough) — proves JP2K path works without a codec
- `sample_files/svs/scan_620_.svs` (270 MB, BigTIFF JPEG, Grundium) — full-walk fixture exercising L18 (no shared JPEGTables)
- `sample_files/svs/svs_40x_bigtiff.svs` (4.8 GB, BigTIFF JPEG, Grundium) — sampled fixture; first BigTIFF SVS in the suite
- `sample_files/ndpi/CMU-1.ndpi` (188 MB) — small NDPI fixture
- `sample_files/ndpi/OS-2.ndpi` (931 MB) — medium NDPI with multiple series + a Map page
- `sample_files/ndpi/Hamamatsu-1.ndpi` (6.6 GB) — **NDPI 64-bit offset extension**; sampled fixture; carries a Map page
- `sample_files/phillips-tiff/Philips-1.tiff` (311 MB, 8 levels) — Hamamatsu-scanned, no associated images
- `sample_files/phillips-tiff/Philips-2.tiff` (872 MB, 10 levels) — 3D Histech-scanned, Macro-only
- `sample_files/phillips-tiff/Philips-3.tiff` (3.1 GB, 9 levels, BigTIFF) — Hamamatsu-scanned, Macro + Label
- `sample_files/phillips-tiff/Philips-4.tiff` (277 MB, 9 levels) — Philips-scanned, exercises sparse-tile blank-tile path heavily
- `sample_files/ome-tiff/Leica-1.ome.tiff` (689 MB, 5 levels, BigTIFF) — single main pyramid + macro
- `sample_files/ome-tiff/Leica-2.ome.tiff` (1.2 GB, 6 levels × 4 main pyramids, BigTIFF) — multi-image OME; exercises the v0.6 multi-image deviation

## Commands

The Makefile bundles every gate. Prefer it over typing the env-var dance manually:

```sh
make test     # go test ./... -race -count=1
make cover    # ≥80% per package; OPENTILE_TESTDIR auto-set
make parity   # batched parity oracle vs Python opentile 0.20.0
make vet      # go vet ./...
make bench    # NDPI per-tile throughput regression gate
```

Direct invocations (when the Makefile-implicit env defaults aren't right):

```sh
# regenerate parity fixtures from real slides (walks svs/ and ndpi/)
OPENTILE_TESTDIR="$PWD/sample_files" \
  go test ./tests -tags generate -run TestGenerateFixtures -generate -v

# byte-parity vs Python opentile 0.20.0 with custom Python interpreter
OPENTILE_ORACLE_PYTHON=/private/tmp/opentile-py/bin/python \
OPENTILE_TESTDIR="$PWD/sample_files" \
  go test ./tests/oracle/... -tags parity -v
```

## Execution mode

Plan execution uses `superpowers:subagent-driven-development`: one fresh implementer subagent per plan task, followed by a spec-compliance review subagent and a code-quality review subagent. Tasks are batched 4–6 at a time; after each batch, execution halts for a controller checkpoint before the next batch begins.
