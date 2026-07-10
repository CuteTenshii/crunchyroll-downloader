# AnimeHeaven (Crunchyroll Downloader)

## What This Is

A CLI tool written in Go that downloads anime episodes from Crunchyroll, decrypts Widevine DRM, and remuxes video + multi-audio + multi-subtitle tracks into MKV files. Now with proper error handling, configurable output, disk-efficient streaming, and CI-backed testing.

## Core Value

Download any anime episode or full season from Crunchyroll into a single playable MKV file with chosen audio and subtitle tracks.

## Requirements

### Validated

- ✓ User can download single episode from `/watch/` URL — v1.0
- ✓ User can download full season/series from `/series/` URL — v1.0
- ✓ User can download batch of URLs from text file — v1.0
- ✓ User can select video quality (1080p, 720p, etc.) — v1.0
- ✓ User can select audio quality — v1.0
- ✓ User can select multiple audio languages (multi-dub) — v1.0
- ✓ User can select multiple subtitle languages — v1.0
- ✓ User can use "all" shorthand for all available audio/subtitle tracks — v1.0
- ✓ Widevine DRM decryption via `.wvd` or `client_id.bin` + `private_key.pem` — v1.0
- ✓ Segments downloaded concurrently with configurable worker pool and retry-with-backoff — v1.0
- ✓ Bearer token auto-refresh on 401 (bounded to one retry) — v1.0
- ✓ Stream cleanup via `DELETE /playback/v1/token/` — v1.0
- ✓ MKV metadata injection (title, show, track, language tags) — v1.0
- ✓ `ja-JP`, `en-US`, `pt-BR` and 24 other locale mappings — v1.0
- ✓ Cross-compilation for Linux/macOS/Windows — v1.0
- ✓ Streaming segment assembly (400MB → ~64KB per-episode RAM) — v1.0
- ✓ HTTP connection reuse with configured transport and timeouts — v1.0
- ✓ All panic() calls replaced with error returns — v1.0
- ✓ Cached Widevine device (sync.Once, no os.ReadDir(".") per request) — v1.0
- ✓ No stack traces shown to user on error — v1.0
- ✓ Graceful SIGINT/SIGTERM — cleanup temp files, release streams — v1.0
- ✓ `CRUNCHYROLL_ETP_RT` environment variable support — v1.0
- ✓ Config file support (`~/.config/animeheaven/config.json`) — v1.0
- ✓ `--output-dir` flag for custom output directory — v1.0
- ✓ Batch URL validation upfront — v1.0
- ✓ FFmpeg availability check at startup — v1.0
- ✓ Explicit Widevine device paths via CLI flags — v1.0
- ✓ `CRUNCHYROLL_CLIENT_AUTH` env var for Basic Auth override — v1.0
- ✓ Season download per-episode progress [Episode N/M] — v1.0
- ✓ Download speed (MB/s) and ETA during segments — v1.0
- ✓ `--quiet` / `--json` output modes — v1.0
- ✓ Season error accumulation with summary — v1.0
- ✓ Structured logging with levels (Info/Warn/Error/Debug/Progress) — v1.0
- ✓ Configurable `--workers` flag for segment concurrency — v1.0
- ✓ GetBaseUrl split into GetVideoBaseUrl/GetAudioBaseUrl — v1.0
- ✓ URL validation fix (`&&` → `||`) — v1.0
- ✓ `url.Parse()` replacing string-splitting — v1.0
- ✓ Bounded token refresh (one retry, no recursion) — v1.0
- ✓ Regex sanitizeFilename (O(n) replaces O(n²)) — v1.0
- ✓ `parseLangs` called once at startup — v1.0
- ✓ `os.CreateTemp` errors checked and propagated — v1.0
- ✓ FFmpeg failures return error (no panic) — v1.0
- ✓ Mux cleanup warnings instead of silent discards — v1.0
- ✓ Empty adaptation set guard — v1.0
- ✓ Widevine discovery errors surfaced — v1.0
- ✓ Comprehensive test suite (unit + integration, 9 packages) — v1.0
- ✓ GitHub Actions CI with lint, vet, test, coverage — v1.0

### Active

- [ ] Resumable downloads — track completed segments in a `.part` state file
- [ ] Concurrent episode downloads within a season
- [ ] Interactive config wizard (`animeheaven setup`)
- [ ] Automatic device ID persistence
- [ ] Download queue with estimated total time
- [ ] Colored terminal output (green ✓ / red ✗ / yellow ⚠)
- [ ] Rate-limit handling (429 responses) with exponential backoff + jitter
- [ ] Disk space check before starting downloads
- [ ] Coverity scan / stricter coverage targets

### Out of Scope

- Support for other streaming services (Funimation, Netflix) — scope too broad
- GUI or web interface — CLI-only by design
- Mobile app — desktop CLI focused
- Real-time streaming / playback — download-only tool
- Cloud storage integration — local filesystem only

## Context

Shipped v1.0 with 5,525 LOC Go across 6 internal packages + testutil.
Tech stack: Go 1.25, golang.org/x/sync (errgroup), golang.org/x/term, gowidevine v0.1.3, go-mpd.
CI: GitHub Actions with go vet, golangci-lint (6 linters), test with -race, coverage reporting.

Current state: 9 test packages, all passing with -race. Zero panic() calls. RAM reduced from 400MB to ~64KB per episode for large downloads.

## Constraints

- **Tech stack**: Go 1.25 — no framework migration planned
- **External dep**: FFmpeg required at runtime for muxing (no alternative)
- **External dep**: gowidevine v0.1.3 for Widevine CDM (pre-v1, API stability risk)
- **External dep**: go-mpd pseudo-version for DASH manifest parsing
- **Compatibility**: Must support Linux, macOS, Windows (cross-compiled binary)
- **Backward compat**: CLI flags and behavior must remain backward compatible

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Keep Go stdlib for CLI | No framework dependency, simple flag parsing sufficient | ✓ Good |
| Use `internal/` packages | Already refactored from flat main — good modularity baseline | ✓ Good |
| Keep FFmpeg for muxing | Battle-tested, no Go-native muxer matches MKV support | ✓ Good |
| Return errors instead of panic | Replace all panic() with error returns throughout codebase | ✓ Good |
| Disk-backed segment assembly | Eliminate 400MB RAM buffering per episode | ✓ Good |
| errgroup for parallel audio | Structured concurrency with fail-fast cancellation | ✓ Good |
| Pointer-field Config struct | Explicit-only JSON overrides with clear precedence | ✓ Good |
| output.Global singleton | Mirrors config pattern, no plumbing through signatures | ✓ Good |
| Table-driven stdlib tests only | No testify dependency (D-03 from Phase 5) | ✓ Good |
| CI continue-on-error for lint | Lint issues don't block PR merging | ✓ Good |

---

*Last updated: 2026-07-10 after v1.0 milestone completion*
