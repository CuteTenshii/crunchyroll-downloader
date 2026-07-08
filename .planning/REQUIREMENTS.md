# Requirements: AnimeHeaven

**Defined:** 2026-07-08
**Core Value:** Download any anime episode or full season from Crunchyroll into a single playable MKV file with chosen audio and subtitle tracks.

## v1 Requirements

### Performance (PERF)

- [x] **PERF-01**: Segments streamed to temp file incrementally during download instead of buffering entire video in `[]byte` — reduce per-episode RAM from 400MB to ~64KB
- [x] **PERF-02**: HTTP transport configured with keep-alive, `MaxIdleConnsPerHost` ≥ 10, and connection reuse for segment downloads — eliminate per-request TCP/TLS handshake
- [x] **PERF-03**: `http.Client.Timeout` set (30s connect, 60s overall) and `context.Context` propagated through all HTTP calls — prevent indefinite hangs
- [ ] **PERF-04**: Parsed MPD manifests cached per `contentId` — avoid redundant fetch/re-parse when downloading multiple audio versions of the same episode
- [ ] **PERF-05**: Widevine device loaded once at startup, cached and reused across license requests — eliminate repeated `os.ReadDir(".")` per audio version
- [ ] **PERF-06**: Audio version manifest fetching and license challenges parallelized via goroutines — reduce multi-dub overhead from sequential to concurrent
- [x] **PERF-07**: `DownloadPart()` and `DownloadSubs()` use configured `http.Client` instead of `http.DefaultClient.Do()` — enable keep-alive, timeouts, and auth for all HTTP paths
- [x] **PERF-08**: Fix file descriptor leak in `getFilename()` — close `*os.File` handle immediately after creating temp file instead of discarding it

### Usability (USAB)

- [ ] **USAB-01**: `CRUNCHYROLL_ETP_RT` environment variable supported as alternative to `--etp-rt` flag (flag takes precedence when both provided)
- [ ] **USAB-02**: Config file support at `~/.config/animeheaven/config.json` for persistent defaults (quality, langs, output dir, workers)
- [ ] **USAB-03**: `--output-dir` flag to specify custom output directory (default: current directory / series title subfolder)
- [ ] **USAB-04**: Batch file URLs validated upfront — report all invalid URLs before starting any downloads, instead of failing mid-batch
- [ ] **USAB-05**: FFmpeg availability checked at startup (before any download work) — clear error if not found on `$PATH`
- [ ] **USAB-06**: `--widevine-device` and `--widevine-key` CLI flags for explicit paths to `.wvd` / `client_id.bin` / `private_key.pem` — stop scanning CWD
- [ ] **USAB-07**: Document hardcoded Basic Auth credential in `auth.go` (public client ID?) and support `CRUNCHYROLL_CLIENT_AUTH` env var as override — prevent breakage if Crunchyroll rotates the credential

### User Experience (UX)

- [x] **UX-01**: All `panic()` calls replaced with proper `error` returns — network blips, API errors, and parsing failures handled gracefully instead of crashing
- [ ] **UX-02**: Season download shows `[Episode 3/24] Title — Downloading... ✓` with per-episode result and cumulative progress
- [ ] **UX-03**: Download speed (MB/s) and estimated time remaining displayed during segment downloads
- [ ] **UX-04**: `--quiet` flag suppresses all progress output (only errors); `--json` flag outputs machine-parseable progress events as NDJSON
- [x] **UX-05**: No stack traces shown to user on error — user-facing messages are clean, actionable, and suggest next steps
- [ ] **UX-06**: Season/batch error accumulation — report total failed episodes at end instead of silent `continue` in `Season()`
- [ ] **UX-07**: Graceful SIGINT/SIGTERM handling — cleanup temp files and release Crunchyroll playback streams on interrupt instead of leaving orphaned state
- [ ] **UX-08**: Structured logging with levels (info, warn, error) and optional JSON output — replace raw `fmt.Printf` scatter

### Quality of Life (QOL)

- [ ] **QOL-01**: Test suite with:
  - Unit tests for `parseLangs`, `sanitizeFilename`, `ExpandTimeline`, `GetBaseUrl`, `BuildUrl`
  - Integration tests for `processURL` (mock HTTP server for Crunchyroll API)
  - Test coverage target ≥ 60% for `internal/media/`, `internal/download/`, `internal/locale/`
- [x] **QOL-02**: `--workers` flag (default 10) to configure segment download concurrency
- [ ] **QOL-03**: `GetBaseUrl` split into `GetVideoBaseUrl` and `GetAudioBaseUrl` — eliminate fragile `isVideoSet` boolean parameter
- [ ] **QOL-04**: Fix URL content ID validation bug: `len(id) < 9 && len(id) > 14` → `len(id) < 9 || len(id) > 14` (`main.go`)
- [ ] **QOL-05**: Replace string-splitting URL parser with `url.Parse()` from stdlib — handle trailing slashes and query params correctly
- [x] **QOL-06**: `DoRequest()` token refresh protected by recursion-depth guard (max 1 re-auth attempt) — prevent stack overflow on expired cookies
- [ ] **QOL-07**: `sanitizeFilename` uses regex (`_{2,}` → `_`) instead of O(n²) `for strings.Contains` loop
- [ ] **QOL-08**: `parseLangs` called once at flag parse time instead of per-URL in batch mode
- [x] **QOL-09**: `os.CreateTemp` errors checked and propagated — no silent discards
- [x] **QOL-10**: FFmpeg merge failures return `error` instead of `panic` — graceful cleanup of partial output
- [x] **QOL-11**: Log warnings instead of silent `_ = os.Remove()` discards in mux cleanup — propagate cleanup failures via log instead of swallowing
- [x] **QOL-12**: Fix `getFilename()` empty return when `set.Representations` is empty — guard against nil/empty adaptation set to prevent downstream panic
- [ ] **QOL-13**: Check and propagate `os.ReadDir(".")` error in `GetWidevineDevice()` instead of silent discard with `_`

## v2 Requirements

### Performance

- **PERF-09**: Resume partial downloads — track completed segments in a `.part` state file
- **PERF-10**: Concurrent episode downloads within a season (worker pool for episodes, not just segments)

### Usability

- **USAB-09**: Interactive config wizard (`animeheaven setup`) — walks user through obtaining `etp_rt`, downloading `.wvd`, setting defaults
- **USAB-10**: Automatic device ID persistence to `~/.config/animeheaven/device_id` — stable device identity across runs

### User Experience

- **UX-09**: Download queue with estimated total time for batch/season downloads
- **UX-10**: Colored terminal output (green ✓ / red ✗ / yellow ⚠)

### Quality of Life

- **QOL-14**: GitHub Actions CI with `go vet`, `golangci-lint`, and test coverage reporting
- **QOL-15**: Rate-limit handling (429 responses) with exponential backoff + jitter across all API calls
- **QOL-16**: Disk space check before starting downloads

## Out of Scope

| Feature | Reason |
|---------|--------|
| GUI / TUI | CLI-only by design — maintain minimal footprint |
| Non-Crunchyroll sources | Single-source focus — adding more sources is a new project |
| Real-time playback / streaming | Download-only tool — no streaming server |
| Cloud storage integration | Local filesystem only — let users sync with their own tooling |
| Automatic `etp_rt` extraction from browser | Security risk — user should consciously provide credentials |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PERF-01 | Phase 1 | Complete |
| PERF-02 | Phase 1 | Complete |
| PERF-03 | Phase 1 | Complete |
| PERF-04 | Phase 2 | Pending |
| PERF-05 | Phase 1 | Pending |
| PERF-06 | Phase 2 | Pending |
| PERF-07 | Phase 1 | Complete |
| PERF-08 | Phase 1 | Complete |
| USAB-01 | Phase 3 | Pending |
| USAB-02 | Phase 3 | Pending |
| USAB-03 | Phase 3 | Pending |
| USAB-04 | Phase 3 | Pending |
| USAB-05 | Phase 3 | Pending |
| USAB-06 | Phase 3 | Pending |
| USAB-07 | Phase 3 | Pending |
| UX-01 | Phase 1 | Complete |
| UX-02 | Phase 4 | Pending |
| UX-03 | Phase 4 | Pending |
| UX-04 | Phase 4 | Pending |
| UX-05 | Phase 1 | Complete |
| UX-06 | Phase 4 | Pending |
| UX-07 | Phase 1 | Pending |
| UX-08 | Phase 4 | Pending |
| QOL-01 | Phase 5 | Pending |
| QOL-02 | Phase 1 | Complete |
| QOL-03 | Phase 2 | Pending |
| QOL-04 | Phase 3 | Pending |
| QOL-05 | Phase 3 | Pending |
| QOL-06 | Phase 1 | Complete |
| QOL-07 | Phase 3 | Pending |
| QOL-08 | Phase 3 | Pending |
| QOL-09 | Phase 1 | Complete |
| QOL-10 | Phase 1 | Complete |
| QOL-11 | Phase 1 | Complete |
| QOL-12 | Phase 1 | Complete |
| QOL-13 | Phase 1 | Pending |

**Coverage:**
- v1 requirements: 37 total
- Mapped to phases: 37
- Unmapped: 0 ✓

---
*Requirements defined: 2026-07-08*
*Last updated: 2026-07-08 after completing plan 01-03*
