# Roadmap: AnimeHeaven

**Created:** 2026-07-08
**Phases:** 5 (expanded: 37 v1 requirements)
**Scope:** Performance, usability, UX, and code quality improvements to the Crunchyroll downloader.

---

## Phase 1: Foundation — Error Handling, HTTP, Memory

**Goal:** Eliminate crashes, reduce RAM, and fix the most dangerous bugs in the download pipeline.
**Progress:** 5/5 Phase 1 plan files complete (01-05 completed 2026-07-08).

| Plan | Req | Description |
|------|-----|-------------|
| 1.1 | UX-01, UX-05, QOL-10, QOL-11 | ✓ Completed 2026-07-08 — Replace all `panic()` with `error` returns — FFmpeg merge, API calls, file I/O, mux cleanup warnings |
| 1.2 | PERF-01 | ✓ Completed 2026-07-08 — Streaming segment assembly writes init/media payloads to temp files and decrypts from disk |
| 1.3 | PERF-02, PERF-03, PERF-07, QOL-06 | ✓ Completed 2026-07-08 — Configured HTTP transport (keep-alive, timeouts, context propagation, `MaxIdleConnsPerHost`) — also fix `DownloadPart`/`DownloadSubs` to use configured client instead of `http.DefaultClient`; bounded 401 refresh to one retry |
| 1.4 | PERF-05, QOL-13 | ✓ Completed 2026-07-08 — Widevine device is loaded once per process, reused across license requests, and discovered from explicit environment or `.env` paths without `os.ReadDir(".")` scanning |
| 1.5 | QOL-02, QOL-09, QOL-12 | ✓ Completed 2026-07-08 — `--workers` flag, checked `os.CreateTemp` errors, and guarded empty adaptation sets |
| 1.6 | QOL-01 | ✓ Completed 2026-07-08 — First useful test batch covers error contracts, HTTP retry/cancellation, segment assembly, Widevine discovery, cleanup/interruption, and CLI parsing |
| 1.7 | PERF-08, QOL-11 | ✓ Completed 2026-07-08 — Closed temp handles in `getFilename()`; mux cleanup warnings were completed in 01-01 |
| 1.8 | UX-07 | ✓ Completed 2026-07-08 — Graceful SIGINT/SIGTERM handling cancels active work, terminates ffmpeg, removes local partial artifacts, and releases Crunchyroll streams on interrupt |

**Details:**
- 1.1 touches every internal package — this is the most invasive change but unblocks everything else
- 1.2 is the highest-impact performance fix (400MB → ~64KB per-episode RAM)
- 1.6 is intentionally last in the phase so tests can validate the refactored code
- 1.7 and 1.8 are low-effort fixes that catch resource leaks and signal safety gaps

---

## Phase 2: Performance — Caching & Parallelism

**Goal:** Optimize multi-dub downloads and eliminate redundant work.
**Progress:** 3/3 Phase 2 plan files created (2026-07-08).

| Plan | Req | Description |
|------|-----|-------------|
| 2.1 | PERF-04 | ✓ Planned 2026-07-08 — MPD manifest cache (sync.RWMutex + map), wired into sequential loop, cache miss/hit/concurrent tests |
| 2.2 | PERF-06 | ✓ Planned 2026-07-08 — errgroup parallel audio fan-out for versions [1..N], fail-fast, deferred stream cleanup, mutex-protected shared state |
| 2.3 | QOL-03 | ✓ Planned 2026-07-08 — GetBaseUrl split into GetVideoBaseUrl/GetAudioBaseUrl with explicit switch/case bandwidth matching |

**Details:**
- Plans 2.1 and 2.3 are Wave 1 (independent, both modify manifest.go + episode.go)
- Plan 2.2 is Wave 2 (depends on 2.1 for MPD cache, also benefits from 2.3 for GetAudioBaseUrl)
- 2.2 depends on Phase 1's error handling (no `panic()` in goroutines) and PERF-05 (cached Widevine device) for concurrent license requests

---

## Phase 3: Usability — Configuration & Validation

**Goal:** Better CLI ergonomics — env vars, config files, validation.

| Plan | Req | Description |
|------|-----|-------------|
| 3.1 | USAB-01, USAB-02 | Env var support (`CRUNCHYROLL_ETP_RT`), config file (`~/.config/animeheaven/config.json`) |
| 3.2 | USAB-03, USAB-04 | `--output-dir` flag, batch URL validation upfront |
| 3.3 | USAB-05, USAB-06 | FFmpeg check at startup, explicit Widevine device paths |
| 3.4 | QOL-04, QOL-05, QOL-07, QOL-08 | Fix URL validation (`&&`→`||`), `url.Parse()` instead of string split, regex sanitize, parseLangs once |
| 3.5 | USAB-07 | Document hardcoded Basic Auth credential and add `CRUNCHYROLL_CLIENT_AUTH` env var override |

---

## Phase 4: User Experience — Progress & Output

**Goal:** Human-friendly and machine-friendly output.

| Plan | Req | Description |
|------|-----|-------------|
| 4.1 | UX-02, UX-06 | Season download progress — `[Episode 3/24] Title — ✓` with per-episode result and cumulative error report at end |
| 4.2 | UX-03 | Download speed (MB/s) and ETA during segment downloads |
| 4.3 | UX-04, UX-08 | `--quiet` and `--json` output modes; structured logging with levels replacing raw `fmt.Printf` |

---

## Phase 5: Testing & Quality

**Goal:** Comprehensive test coverage and CI integration.

| Plan | Req | Description |
|------|-----|-------------|
| 5.1 | QOL-01 | Full test suite: unit tests for all internal packages, integration tests with mock HTTP |
| 5.2 | QOL-01 | CI pipeline with `go vet`, lint, and coverage reporting |

---

## Summary

| Phase | Focus | Plans | Req Count | Est. Effort |
|-------|-------|-------|-----------|-------------|
| 1 | Foundation (errors, HTTP, memory) | 8 | 18 | High |
| 2 | Performance (caching, parallelism) | 3 | 3 | Medium |
| 3 | Usability (CLI, config, validation) | 5 | 11 | Medium |
| 4 | UX (progress, output) | 3 | 5 | Low |
| 5 | Testing & CI | 2 | 1 | Medium |
---

*Roadmap created: 2026-07-08*
*Last updated: 2026-07-08 after completing plan 01-05*
