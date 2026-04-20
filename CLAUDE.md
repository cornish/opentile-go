# opentile-go

Pure-Go port of [imi-bigpicture/opentile](https://github.com/imi-bigpicture/opentile) (Apache 2.0, Sectra AB). Reads tiles from WSI (whole-slide imaging) TIFF files used in digital pathology.

## Current milestone — v0.1

- **Scope:** Aperio SVS tiled-level passthrough only.
- **Deferred:** NDPI (v0.2), SVS associated images — label/overview/thumbnail (v0.3), SVS corrupt-edge reconstruct fix (v1.0), BigTIFF (ship when first format needs it), `internal/jpeg` marker work (ships with NDPI).
- **Design:** `docs/superpowers/specs/2026-04-19-opentile-go-design.md`
- **Plan:** `docs/superpowers/plans/2026-04-19-opentile-go-v01.md`
- **Work branch:** `feat/v0.1`

## Invariants

- **Pure Go, no cgo.** The SVS v0.1 tile hot path is TIFF parsing + byte-range reads; no image codec is required. Defer any cgo consideration until profiling on realistic slides justifies it.
- **Direct port under Apache 2.0** with attribution retained in `NOTICE`. Not affiliated with or endorsed by Sectra AB or the BigPicture project.
- **Parity with upstream is the correctness bar.** Upstream's pytest cases are ported to Go tests; a fixture-backed integration suite compares tile bytes against a committed snapshot. An opt-in `//go:build parity` harness that shells out to Python opentile is planned for v0.2.
- **Lock-free hot path.** All internal caches (parsed IFDs, per-tile offset/length arrays, metadata) are populated at `Open()` time and immutable thereafter. `Tile()` is safe to call concurrently from many goroutines.

## Conventions

- Module path: `github.com/tcornish/opentile-go`
- Go 1.23+ (for `iter.Seq2`)
- `internal/tiff` and `internal/jpeg` are internal — shaped for opentile's needs, not general-purpose libraries
- Format subpackages (`formats/svs/`, `formats/ndpi/`, …) are public; `formats/all` is the umbrella registration package
- `io.ReaderAt` + `int64` size is the core input (stdlib `*os.File` satisfies concurrent-use semantics)
- Public tile methods: `Level.Tile(x, y int)` returns raw compressed bytes; `Level.TileReader(x, y)` streams via `io.SectionReader`; `Level.Tiles(ctx)` is serial row-major via `iter.Seq2`

## Sample slides

Local slides live in `/sample_files/` (gitignored). v0.1 uses:
- `sample_files/svs/CMU-1-Small-Region.svs` (1.9 MB, JPEG) — primary fixture
- `sample_files/svs/CMU-1.svs` (177 MB, JPEG) — full-slide fixture
- `sample_files/svs/JP2K-33003-1.svs` (63 MB, JPEG 2000 passthrough) — proves JP2K path works without a codec
- `sample_files/ndpi/*.ndpi` — reserved for v0.2

## Commands

```sh
# unit + existing tests
go test ./... -race

# integration test against real slides (requires OPENTILE_TESTDIR pointing at a
# directory containing one or more of the committed fixture slides)
OPENTILE_TESTDIR="$PWD/sample_files/svs" go test ./tests/... -v

# regenerate parity fixtures from real slides
OPENTILE_TESTDIR="$PWD/sample_files/svs" \
  go test ./tests -tags generate -run TestGenerateFixtures -generate -v
```

## Execution mode

Plan execution uses `superpowers:subagent-driven-development`: one fresh implementer subagent per plan task, followed by a spec-compliance review subagent and a code-quality review subagent. Tasks are batched 4–6 at a time; after each batch, execution halts for a controller checkpoint before the next batch begins.
