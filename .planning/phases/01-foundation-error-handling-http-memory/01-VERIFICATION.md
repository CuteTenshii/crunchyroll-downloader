---
phase: 01-foundation-error-handling-http-memory
verified: 2026-07-09T07:37:03Z
status: passed
next_action: "Run end-to-end human UAT for OS signal interruption and shutdown cleanup behavior."
next_command: "Manual UAT: start a real or mocked download, send SIGINT/SIGTERM, and confirm local artifacts are removed and shutdown is bounded."
score: "26/30 must-haves verified"
behavior_unverified: 4
overrides_applied: 0
deferred:

  - truth: "QOL-01 strict coverage target >= 60% for internal/media, internal/download, and internal/locale"
    addressed_in: "Phase 5"
    evidence: "ROADMAP Phase 5 goal is comprehensive test coverage and CI integration; Plan 5.1 is QOL-01 full test suite."
behavior_unverified_items:

  - truth: "D-18: SIGINT and SIGTERM cancel active work instead of leaving partial state behind."
    test: "Start the CLI against a real or mocked long-running download, send SIGINT and SIGTERM, and observe cancellation reaching active work."
    expected: "The process exits cleanly without panic, active work is canceled, and no partial local state remains."
    why_human: "Code uses signal.NotifyContext and passes ctx downward, but no automated test sends real OS signals through main."

  - truth: "D-20: Temporary files are always removed on interruption."
    test: "Interrupt during subtitle, audio, video, and mux stages while temp files exist."
    expected: "All tracked temp files are removed after interruption."
    why_human: "Unit tests cover cleanup helpers and segment failure cleanup, but not an end-to-end interrupted episode path."

  - truth: "D-21: DeleteStream failures do not block local shutdown."
    test: "Force DeleteStream to fail or hang during interrupted shutdown."
    expected: "Local cleanup still completes and shutdown remains bounded by the cleanup timeout."
    why_human: "The defer uses a 10s background cleanup context, but no test exercises DeleteStream failure timing during shutdown."

  - truth: "D-22: Any partial episode artifacts are removed after interruption."
    test: "Interrupt after a partial output file has been created."
    expected: "The partial output and temp artifacts are removed."
    why_human: "The cleanup function is tested directly, but the interrupt-triggered path through Episode is not exercised."
human_verification:

  - test: "SIGINT/SIGTERM end-to-end cancellation"
    expected: "A running download exits cleanly without panic, cancels active HTTP/media/mux work, and leaves no partial local artifacts."
    why_human: "Requires real OS signal delivery through the CLI entrypoint and a long-running download or mock harness."

  - test: "Interrupted shutdown with DeleteStream failure"
    expected: "A remote DeleteStream failure or hang is reported/bounded and does not prevent local artifact cleanup."
    why_human: "Requires an API failure/hang during deferred cleanup; static checks cannot prove shutdown timing."
---

# Phase 1: Foundation - Error Handling, HTTP, Memory Verification Report

**Phase Goal:** Eliminate crashes, reduce RAM, and fix the most dangerous bugs in the download pipeline.  
**Verified:** 2026-07-09T07:37:03Z  
**Status:** human_needed  
**Re-verification:** No - initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|---|---|---|
| 1 | D-01: Episode failures return errors instead of panicking. | VERIFIED | `internal/download/episode.go` returns errors; `TestEpisodeReturnsErrorForUnavailableAudioLocale` passes. |
| 2 | D-01: Season runs continue after a single episode failure once cleanup finishes. | VERIFIED | `runSeason` accumulates failures and continues; `TestRunSeasonContinuesAfterEpisodeFailure` passes. |
| 3 | D-02: User-facing errors stay short, clear, and actionable. | VERIFIED | No panic traces remain; `main.go` prints concise errors from returned failures. |
| 4 | D-03: Cleanup failures are warnings when the main work succeeded. | VERIFIED | `mux.warnRemove` prints warnings after successful mux; `TestMergeEverythingWarnsButSucceedsWhenCleanupFails` passes. |
| 5 | D-04: Expected control flow does not use panic(). | VERIFIED | `rg "panic\\(" . --glob '*.go'` returned zero matches. |
| 6 | D-05: Shared HTTP client uses configured transport with keep-alive and idle reuse. | VERIFIED | `configuredHTTPClient` sets transport, timeouts, and `MaxIdleConnsPerHost: 20`; test passes. |
| 7 | D-05: HTTP calls carry context and timeouts from main down through API calls. | VERIFIED | `main` creates root ctx; API request methods use `newRequest(ctx, ...)`; cancellation test passes. |
| 8 | D-06: 401 refresh happens once and only once before returning an error. | VERIFIED | `Client.Do` has one retry branch; retry success and second-401 failure tests pass. |
| 9 | D-08: Cancellation propagates from main through API and download entrypoints. | VERIFIED | Context is threaded through `processURL`, `Episode`, `Season`, API, media, DRM, and mux; request cancellation test passes. |
| 10 | D-04: Retry path remains explicit and bounded. | VERIFIED | `Client.Do` uses one explicit retry, no recursion. |
| 11 | D-09: Segment assembly writes init and media segments to a temp file before decrypting. | VERIFIED | `DownloadParts` writes init and segment data to `crdl-encrypted-*.mp4`, then opens it for decrypt. |
| 12 | D-10: Partial segment failure aborts the download and cleans all temporaries. | VERIFIED | `DownloadParts` cancels on worker error and defers encrypted temp removal; failure cleanup test passes. |
| 13 | D-11: Segment progress remains segment-based while payload is streamed to disk. | VERIFIED | Progress uses `done.Add(1)` and prints downloaded segment count. |
| 14 | D-12: DownloadParts owns temp-file creation, write, decrypt, and cleanup. | VERIFIED | `DownloadParts` creates encrypted and output temp files, writes, decrypts, closes, and removes on error. |
| 15 | D-07: Segment and subtitle downloads use configured client path instead of http.DefaultClient. | VERIFIED | `DownloadPart` and `DownloadSubs` accept `httpDoer`; default HTTP client grep returned zero matches. |
| 16 | D-02: Temp-file creation, file closing, and empty-adaptation-set checks return errors. | VERIFIED | `createTempFilename` returns errors and closes handles; manifest guards and tests pass. |
| 17 | D-08: Segment worker count is configurable. | VERIFIED | `--workers` flag flows from `main` into `Episode`, `Season`, and `DownloadParts`; season test asserts worker propagation. |
| 18 | D-13: Widevine device is loaded once per process and reused. | VERIFIED | `GetWidevineDevice` uses `sync.Once`; cache test passes. |
| 19 | D-14: Device paths come from environment variables loaded from .env. | VERIFIED | `discoverWidevineDeviceConfig` merges env and `.env`; precedence tests pass. |
| 20 | D-15: client_id.bin and private_key.pem are valid only as a pair. | VERIFIED | Incomplete raw pair returns an error; raw-pair tests pass. |
| 21 | D-16: A packaged .wvd file wins over raw pair when both are present. | VERIFIED | `.wvd` path returns before raw pair; precedence tests pass. |
| 22 | D-17: Missing device config returns one explicit accepted-formats error. | VERIFIED | `missingWidevineDeviceError` is shared by discovery and license fallback; test passes. |
| 23 | D-18: SIGINT and SIGTERM cancel active work instead of leaving partial state behind. | PRESENT_BEHAVIOR_UNVERIFIED | Present via `signal.NotifyContext` and context wiring; no test sends real OS signals through `main`. |
| 24 | D-19: ffmpeg is terminated on interruption and partial output is removed. | VERIFIED | `MergeEverything` uses `exec.CommandContext`; cancellation cleanup test passes. |
| 25 | D-20: Temporary files are always removed on interruption. | PRESENT_BEHAVIOR_UNVERIFIED | Cleanup paths exist and helper tests pass; no interrupted episode test covers all temp stages. |
| 26 | D-21: DeleteStream failures do not block local shutdown. | PRESENT_BEHAVIOR_UNVERIFIED | Deferred cleanup uses a 10s background context; no shutdown timing/failure test exercises it. |
| 27 | D-22: Any partial episode artifacts are removed after interruption. | PRESENT_BEHAVIOR_UNVERIFIED | `cleanupEpisodeArtifacts` removes output and temps; interrupt-triggered Episode path is not tested. |
| 28 | D-23: Test investment focuses on fragile points. | VERIFIED | Tests cover retry, cancellation, mux cleanup, segment temp cleanup, DRM discovery, season continuation, and URL parsing. |
| 29 | D-24: First test batch covers named Phase 1 areas. | VERIFIED | `go test ./...` passes across main, api, download, drm, media, and mux packages. |
| 30 | D-25: Phase sets a minimal useful coverage bar, not strict high target. | VERIFIED | Phase tests are focused; stricter QOL-01 coverage target is deferred to Phase 5. |

**Score:** 26/30 truths verified (4 present, behavior-unverified)

### Deferred Items

| # | Item | Addressed In | Evidence |
|---|---|---|---|
| 1 | QOL-01 strict coverage target >= 60% for `internal/media`, `internal/download`, and `internal/locale` | Phase 5 | ROADMAP Phase 5 includes QOL-01 full test suite and CI coverage. Current coverage: media 52.7%, download 26.0%, locale no tests. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|---|---|---|---|
| `main.go` | Root context, signal handling, workers flag, API client wiring | VERIFIED | Contains `signal.NotifyContext`, `api.NewWithContext`, `processURL(ctx, ...)`, and `--workers`. |
| `internal/api/client.go` | Configured transport, timeout, bounded auth retry | VERIFIED | Uses configured `http.Transport`, 60s client timeout, one 401 retry. |
| `internal/api/episode.go`, `season.go`, `license.go`, `manifest.go`, `auth.go` | Context-aware API calls | VERIFIED | API methods construct context-aware requests and call `Client.Do`. |
| `internal/download/episode.go` | Episode error returns, cleanup, client/media/mux wiring | VERIFIED | Returns errors, tracks temp files, defers cleanup, calls media and mux with ctx. |
| `internal/download/season.go` | Continue-on-error season loop | VERIFIED | Accumulates failures into `SeasonError` after trying all episodes. |
| `internal/media/segment.go` | Disk-backed segment assembly and configured client usage | VERIFIED | Uses injected doer, temp encrypted file, worker pool, cleanup. |
| `internal/media/manifest.go` | Adaptation-set guards | VERIFIED | Rejects nil/empty/malformed adaptation sets. |
| `internal/drm/drm.go` | Cached deterministic Widevine discovery | VERIFIED | Uses `sync.Once`, env/`.env` lookup, `.wvd` precedence, raw pair validation. |
| `internal/mux/mux.go` | ffmpeg error/cancellation cleanup | VERIFIED | Uses `exec.CommandContext`, removes partial output on failure, warnings after success. |
| Test files | Focused regression coverage | VERIFIED | Required test files exist and `go test ./...` passes. |

### Key Link Verification

| From | To | Via | Status | Details |
|---|---|---|---|---|
| `main.go` | `internal/api/client.go` | `api.NewWithContext(ctx, *etpRt)` | VERIFIED | Root context reaches client initialization. |
| `main.go` | `internal/download/episode.go` / `season.go` | `processURL(ctx, client, ...)` passes ctx and workers | VERIFIED | Episode and season calls include ctx and `*workers`. |
| `internal/download/season.go` | `internal/download/episode.go` | `runSeason(..., Episode)` | VERIFIED | Per-episode errors are logged and accumulated without breaking the loop. |
| `internal/download/episode.go` | `internal/media/segment.go` | `media.DownloadParts(ctx, client, ..., workers)` | VERIFIED | Worker budget and configured API client are passed into media downloads. |
| `internal/download/episode.go` | `internal/mux/mux.go` | `mux.MergeEverything(ctx, ...)` | VERIFIED | Mux receives the cancellation context and returns errors. |
| `internal/drm/drm.go` | `internal/api/license.go` | `client.SendChallenge(ctx, ...)` | VERIFIED | Cached device is used to build challenge; license request uses API client path. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|---|---|---|---|---|
| `main.go` | `ctx`, `workers`, API client | CLI flags and `signal.NotifyContext` | Yes | FLOWING |
| `internal/download/episode.go` | temp file paths, active streams, media tracks | API playback, manifest, media downloads | Yes | FLOWING |
| `internal/media/segment.go` | segment bytes | Injected HTTP client requests | Yes | FLOWING |
| `internal/drm/drm.go` | Widevine device config | env vars and `.env` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|---|---|---|---|
| Full project tests | `rtk test env GOCACHE=/tmp/go-build go test ./...` | all packages pass | PASS |
| Coverage check for QOL-01 strict target | `rtk test env GOCACHE=/tmp/go-build go test ./internal/media ./internal/download ./internal/locale -cover` | media 52.7%, download 26.0%, locale no tests | DEFERRED |
| Panic removal | `rtk grep -n "panic\\(" . --glob '*.go'` | zero matches | PASS |
| Default HTTP client bypass | `rtk grep -n "http\\.DefaultClient|http\\.Get\\(|http\\.Post\\(" internal main.go` | zero matches | PASS |

### Probe Execution

No phase probes were declared or discovered.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|---|---|---|---|---|
| PERF-01 | 01-03 | Stream segments to temp file incrementally | SATISFIED | `DownloadParts` writes encrypted temp file before decrypt. |
| PERF-02 | 01-02 | Configured HTTP transport and keep-alive | SATISFIED | Transport settings and test coverage present. |
| PERF-03 | 01-02 | HTTP timeout and context propagation | SATISFIED | `http.Client.Timeout`, context-aware requests, cancellation test. |
| PERF-05 | 01-04 | Widevine device cached and reused | SATISFIED | `sync.Once` cache and test coverage. |
| PERF-07 | 01-02, 01-03 | Media downloads use configured client | SATISFIED | Injected HTTP doer; default client grep zero. |
| PERF-08 | 01-03 | Close temp handles in `getFilename` | SATISFIED | `createTempFilename` closes file and propagates errors. |
| UX-01 | 01-01 | Replace panic with error returns | SATISFIED | Zero `panic()` matches in Go files. |
| UX-05 | 01-01 | Clean user-facing errors | SATISFIED | Errors are returned and printed without stack traces. |
| UX-07 | 01-05 | Graceful SIGINT/SIGTERM cleanup | NEEDS HUMAN | Signal context wiring exists, but full OS signal UAT is required. |
| QOL-01 | 01-05 | First useful test suite and coverage target | PARTIAL/DEFERRED | Focused tests pass; strict 60% package coverage target is Phase 5 work. |
| QOL-02 | 01-03 | Configurable workers | SATISFIED | `--workers` flag flows into `DownloadParts`. |
| QOL-06 | 01-02 | Bounded token refresh | SATISFIED | One 401 retry tests pass. |
| QOL-09 | 01-03 | `os.CreateTemp` errors checked | SATISFIED | Temp creation errors returned and tested. |
| QOL-10 | 01-01 | ffmpeg failures return errors | SATISFIED | `MergeEverything` returns errors; failure cleanup test passes. |
| QOL-11 | 01-01, 01-05 | Mux cleanup warnings | SATISFIED | Warning path tested. |
| QOL-12 | 01-03 | Empty adaptation set guard | SATISFIED | Manifest and filename guard tests pass. |
| QOL-13 | 01-04 | Widevine discovery errors surfaced | SATISFIED | Current-dir scanning removed; explicit config errors tested. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| None | - | - | - | No TODO/FIXME/XXX/placeholder markers or panic calls found in changed Go files. |

### Human Verification Required

### 1. SIGINT/SIGTERM End-to-End Cancellation

**Test:** Start a real or mocked long-running download and send SIGINT, then SIGTERM.  
**Expected:** Active work is canceled, ffmpeg is stopped if running, process exits cleanly, and partial local artifacts are removed.  
**Why human:** No automated test sends real OS signals through `main`.

### 2. Interrupted Shutdown With DeleteStream Failure

**Test:** Force the Crunchyroll stream cleanup request to fail or hang during interrupted shutdown.  
**Expected:** Failure is reported or bounded, local cleanup still completes, and shutdown is not blocked indefinitely.  
**Why human:** Static checks cannot prove remote cleanup timing under interruption.

## Gaps Summary

No blocking implementation gaps were found for Phase 1's foundation goal. The phase is present and wired, and automated tests pass. Human UAT is still required for the OS signal and shutdown timing paths. The stricter QOL-01 coverage target is not met in Phase 1, but ROADMAP explicitly carries full QOL-01 test-suite work in Phase 5, so it is recorded as deferred rather than a Phase 1 blocker.

---

_Verified: 2026-07-09T07:37:03Z_  
_Verifier: the agent (gsd-verifier)_
