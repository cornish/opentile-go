# opentile-go

Direct Go port of [imi-bigpicture/opentile](https://github.com/imi-bigpicture/opentile) (Apache 2.0, Sectra AB) with one cgo dependency (libjpeg-turbo, narrowly scoped to `internal/jpegturbo/`). Reads tiles from WSI (whole-slide imaging) TIFF files used in digital pathology.

## Current milestone — v0.8

- **Scope:** Iris File Extension (IFE) v1.0 support — **the first non-TIFF format opentile-go reads**, and the first format with no Python or external-binary parity oracle. New `formats/ife/` package (~600 LoC reader + tests); new `FormatFactory.SupportsRaw` + `OpenRaw` + `RawUnsupported` base for non-TIFF dispatch; new `CompressionAVIF` + `CompressionIRIS` enum values; new `ErrSparseTile` sentinel. One real fixture (`cervix_2x_jpeg.iris`, 2.16 GB, JPEG-encoded, 9 layers, 126,976 × 88,576 px native) round-trips cleanly through `opentile.OpenFile`.
- **API extension:** `FormatFactory.SupportsRaw(io.ReaderAt, int64) bool` + `FormatFactory.OpenRaw(r, size, *Config) (Tiler, error)` — additive interface evolution. Non-TIFF formats override both; TIFF-based formats embed `RawUnsupported` for default `false` / `ErrUnsupportedFormat`. `opentile.Open` walks `SupportsRaw` *before* `tiff.Open`, so non-TIFF files never get parsed as TIFF. Backward-compat verified across all 17 packages with `-race`.
- **Active limitations:** L4, L5, L14 (Permanent — carried from v0.6) plus L19, L20 (v0.7 BIF work items still deferred — fixture- or research-driven), L23 (IFE cross-tool parity vs `tile_server_iris` — v0.9+, trigger-driven), L24 (AVIF + Iris-proprietary tile decode — Permanent, byte-passthrough by design), L25 (IFE ANNOTATIONS block parsing — v0.9+, fixture-driven). L22 (METADATA block parsing) was retired by the v0.8 mid-milestone metadata closeout — full reader now ships for METADATA + ATTRIBUTES + IMAGE_ARRAY + ICC_PROFILE.
- **Deviations from upstream Python opentile** (canonical list at `docs/deferred.md §1a`): everything from v0.7 plus two v0.8 entries: non-TIFF dispatch path (architectural — backward-compat additive via `RawUnsupported`); `TILE_TABLE.x_extent` / `y_extent` ignored on IFE (spec-doc-vs-fixture mismatch — values match tile counts, not pixels as spec claims).
- **Correctness bar:** IFE has **no external parity oracle**. v0.7's tifffile + opentile-py oracles can't read IFE; openslide doesn't either. Coverage is layered: sample-tile SHA fixtures (`tests/fixtures/cervix_2x_jpeg.ife.json` via `TestSlideParity`) lock in opentile-go's own output; synthetic-IFE-writer tests in `formats/ife/synthetic_test.go` catch reader bugs without the real fixture; `tests/parity/ife_geometry_test.go` pins per-fixture geometry. Cross-tool divergence (tile bytes mismatch with `tile_server_iris`) is debugged from scratch when it surfaces.
- **Deferred:** R4 (SVS corrupt-edge reconstruct) + R9 (JP2K decode/encode) parked at [#1](https://github.com/cornish/opentile-go/issues/1). R6 (3DHistech TIFF) parked at [#2](https://github.com/cornish/opentile-go/issues/2); R15 (Sakura SVSlide) parked at [#3](https://github.com/cornish/opentile-go/issues/3). v0.9 candidates: L19 / L20 BIF closeout, IFE METADATA (L22), IFE cross-tool parity (L23), or another non-TIFF format (DICOM-WSI). TBD based on real-slide demand.
- **Design:** `docs/superpowers/specs/2026-04-29-opentile-go-ife-design.md`
- **Plan:** `docs/superpowers/plans/2026-04-29-opentile-go-v08-ife.md`
- **Reference spec:** `sample_files/ife/ife-format-spec-for-opentile-go.md`
- **Work branch:** `feat/v0.8`

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
- `sample_files/ventana-bif/Ventana-1.bif` (227 MB) — DP 200 spec-compliant; tifffile parity oracle target
- `sample_files/ventana-bif/OS-1.bif` (3.6 GB) — legacy iScan Coreo; sampled fixture
- `sample_files/ife/cervix_2x_jpeg.iris` (2.16 GB, 9 levels, JPEG) — first non-TIFF fixture; downloaded from Iris's public S3 bucket; SHA256 `b080859913d2…`. Sampled fixture (cervix is too large for full-walk under the 5 MB per-fixture cap)

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
