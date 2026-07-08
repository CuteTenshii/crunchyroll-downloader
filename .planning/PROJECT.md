# AnimeHeaven (Crunchyroll Downloader)

## What This Is

A CLI tool written in Go that downloads anime episodes from Crunchyroll, decrypts Widevine DRM, and remuxes video + multi-audio + multi-subtitle tracks into MKV files. Built for users who want offline local copies of content they have legitimate access to via a Crunchyroll Premium account.

## Core Value

Download any anime episode or full season from Crunchyroll into a single playable MKV file with chosen audio and subtitle tracks.

## Requirements

### Validated

- ✓ User can download single episode from `/watch/` URL — existing
- ✓ User can download full season/series from `/series/` URL — existing
- ✓ User can download batch of URLs from text file — existing
- ✓ User can select video quality (1080p, 720p, etc.) — existing
- ✓ User can select audio quality — existing
- ✓ User can select multiple audio languages (multi-dub) — existing
- ✓ User can select multiple subtitle languages — existing
- ✓ User can use "all" shorthand for all available audio/subtitle tracks — existing
- ✓ Widevine DRM decryption via `.wvd` or `client_id.bin` + `private_key.pem` — existing
- ✓ Segments downloaded concurrently with 10-worker pool and retry-with-backoff — existing
- ✓ Bearer token auto-refresh on 401 — existing
- ✓ Stream cleanup via `DELETE /playback/v1/token/` — existing
- ✓ MKV metadata injection (title, show, track, language tags) — existing
- ✓ `ja-JP`, `en-US`, `pt-BR` and 24 other locale mappings — existing
- ✓ Cross-compilation for Linux/macOS/Windows — existing

### Active

- [ ] **PERF-01**: Streaming segment assembly — write segments incrementally instead of buffering entire video in RAM
- [ ] **PERF-02**: HTTP connection reuse via configured transport — reduce TCP/TLS overhead per segment
- [ ] **PERF-03**: HTTP timeouts and context propagation — prevent hangs on stalled connections
- [ ] **PERF-04**: Cache parsed MPD manifests per contentId — avoid redundant re-fetch/re-parse for multi-dub
- [ ] **PERF-05**: Cache Widevine device at startup — avoid `os.ReadDir(".")` per license request
- [ ] **PERF-06**: Concurrent manifest fetching for audio versions — parallelize independent license flows
- [ ] **PERF-07**: Fix `http.DefaultClient` bypass in segment/subtitle downloads
- [ ] **PERF-08**: Fix file descriptor leak in `getFilename()`
- [ ] **USAB-01**: Support `CRUNCHYROLL_ETP_RT` environment variable for `etp_rt` cookie
- [ ] **USAB-02**: Config file support (`~/.config/animeheaven/config.json`) for persistent settings
- [ ] **USAB-03**: `--output-dir` flag for custom output directory
- [ ] **USAB-04**: Validate batch file URLs upfront before starting downloads
- [ ] **USAB-05**: Validate FFmpeg availability at startup
- [ ] **USAB-06**: Explicit Widevine device paths via CLI flags instead of scanning CWD
- [ ] **USAB-07**: Document hardcoded Basic Auth credential, add env var fallback
- [ ] **UX-01**: Graceful error handling — replace `panic()` with error returns throughout
- [ ] **UX-02**: Season download progress — show "Episode X of Y" with per-episode status
- [ ] **UX-03**: Download speed / ETA estimation during segment downloads
- [ ] **UX-04**: `--quiet` / `--json` output mode for non-interactive use
- [ ] **UX-05**: No stack traces shown to user on error
- [ ] **UX-06**: Season/batch error accumulation — report total failures at end
- [ ] **UX-07**: Graceful SIGINT/SIGTERM handling — cleanup streams on interrupt
- [ ] **UX-08**: Structured logging with levels replacing raw `fmt.Printf`
- [ ] **QOL-01**: Add comprehensive test suite (unit + integration)
- [ ] **QOL-02**: Configurable `--workers` flag for segment concurrency
- [ ] **QOL-03**: Separate `GetBaseUrl` by media type instead of `isVideoSet` boolean
- [ ] **QOL-04**: Fix URL validation bug — `&&` should be `||` in content ID length check
- [ ] **QOL-05**: Proper URL parsing via `url.Parse()` instead of string splitting
- [ ] **QOL-06**: Remove recursive retry in `DoRequest()` — add recursion-depth guard
- [ ] **QOL-07**: Sanitize filename with regex instead of O(n²) loop
- [ ] **QOL-08**: `parseLangs` called once at flag parse time instead of per-URL
- [ ] **QOL-09**: `os.CreateTemp` errors checked and propagated
- [ ] **QOL-10**: FFmpeg merge failures return `error` instead of `panic`
- [ ] **QOL-11**: Log warnings instead of silent `_ = os.Remove()` discards in mux
- [ ] **QOL-12**: Guard against empty adaptation set in `getFilename()`
- [ ] **QOL-13**: Check `os.ReadDir(".")` error in `GetWidevineDevice()`

### Out of Scope

- Support for other streaming services (Funimation, Netflix) — scope too broad for this tool
- GUI or web interface — CLI-only by design
- Mobile app — desktop CLI focused
- Real-time streaming / playback — download-only tool

## Context

AnimeHeaven is a mature Go CLI tool (v1.2.0) with a Crunchyroll-focused architecture. It was recently refactored from a flat `package main` into 6 internal packages (`internal/api/`, `internal/download/`, `internal/drm/`, `internal/media/`, `internal/mux/`, `internal/locale/`). The codebase has comprehensive codebase mapping artifacts but no test suite, no formal error handling strategy (uses `panic()` pervasively), and several performance bottlenecks in its segment assembly and HTTP client usage.

The main pain points users experience today:
- High memory usage for large episodes (`DownloadParts` buffers everything in RAM)
- `panic()` crashes on any network blip during batch/season downloads
- No resume support — interrupted downloads restart entirely
- `etp_rt` cookie exposed via CLI args (process list, shell history)
- No progress feedback for season downloads

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
| Keep Go stdlib for CLI | No framework dependency, simple flag parsing sufficient | — Pending |
| Use `internal/` packages | Already refactored from flat main — good modularity baseline | ✓ Good |
| Keep FFmpeg for muxing | Battle-tested, no Go-native muxer matches MKV support | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---

*Last updated: 2026-07-08 after adding Evolution section and 9 missing improvement requirements*
