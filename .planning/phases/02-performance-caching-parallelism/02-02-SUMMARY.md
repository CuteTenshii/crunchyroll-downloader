---
phase: 02-performance-caching-parallelism
plan: 02
subsystem: download-engine
tags: errgroup, parallelism, mutex, audio, concurrency, go

requires:
  - phase: 02-performance-caching-parallelism
    plan: 01
    provides: MPD manifest cache (GetCachedManifest/SetCachedManifest)
  - phase: 02-performance-caching-parallelism
    plan: 03
    provides: GetVideoBaseUrl/GetAudioBaseUrl split functions
  - phase: 01-foundation-error-handling-http-memory
    provides: error-returning functions, context propagation, cached Widevine device

provides:
  - Parallel audio version manifest fetching and license challenges via errgroup (PERF-06)
  - Phase A: Video + first audio sequential (i==0) — unchanged behavior
  - Phase B: Remaining audio versions [1..N] parallel with fail-fast errgroup
  - Phase C: Deferred bulk stream cleanup for all versions (no per-version DeleteStream)
  - Mutex-protected shared state (activeStreams, tempFiles, audioTracks)
  - Test helper NewTestClient for cross-package test client construction

affects:
  - Phase 5: Testing (mock HTTP server for full parallel integration tests)

tech-stack:
  added:
    - "golang.org/x/sync v0.22.0" — errgroup for structured concurrency with context cancellation
    - "sync.Mutex" (stdlib) — protects shared state from concurrent goroutine writes
  patterns:
    - errgroup.WithContext for fan-out with fail-fast: first error cancels sibling goroutines via derived gctx
    - Cache-first manifest access inside parallel goroutines using GetCachedManifest/SetCachedManifest
    - Mutex-protected slice append for audioTracks and tempFiles across goroutines
    - Three-phase structure: sequential (i==0) → parallel (errgroup [1..N]) → deferred cleanup

key-files:
  created:
    - internal/api/client_helper.go — NewTestClient exported test helper
  modified:
    - go.mod — added golang.org/x/sync v0.22.0 (direct dependency)
    - go.sum — integrity hashes for v0.22.0
    - internal/download/episode.go — restructured audio loop into Phase A/B/C
    - internal/download/episode_test.go — added 3 test functions

key-decisions:
  - "errgroup fan-out for audio versions [1..N] with fail-fast — any goroutine error cancels all siblings via gctx"
  - "Per-version DeleteStream removed from loop body — deferred bulk cleanup handles all streams after parallel work completes (D-12, D-13)"
  - "Shared state (activeStreams, tempFiles, audioTracks) protected by sync.Mutex in goroutines"
  - "Cache-hit path token lookup reads from activeStreams with mutex protection"
  - "Single version (len(versions)==1): no errgroup created, pure sequential path"
  - "NewTestClient helper added to api package — enables downstream test packages to construct minimal clients without real credentials"

patterns-established:
  - "Three-phase audio loop: Phase A (sequential single version) → Phase B (parallel errgroup for [1..N]) → Phase C (deferred cleanup)"
  - "gctx always used inside g.Go closures — never parent ctx — to ensure proper sibling cancellation"

requirements-completed:
  - PERF-06

coverage:
  - id: D1
    description: "Single audio version skips errgroup — sequential path only"
    requirement: PERF-06
    verification:
      - kind: unit
        ref: internal/download/episode_test.go#TestEpisodeSingleVersion
        status: pass
    human_judgment: false
  - id: D2
    description: "Multiple audio versions trigger errgroup fan-out for [1..N] with fail-fast"
    requirement: PERF-06
    verification:
      - kind: unit
        ref: internal/download/episode_test.go#TestEpisodeParallelAudio
        status: pass
    human_judgment: false
  - id: D3
    description: "Unavailable audio locale returns error before parallel work begins"
    requirement: PERF-06
    verification:
      - kind: unit
        ref: internal/download/episode_test.go#TestEpisodeParallelAudioZeroVersions
        status: pass
    human_judgment: false
  - id: D4
    description: "Per-version DeleteStream removed — deferred cleanup handles all streams"
    verification:
      - kind: integration
        ref: go build ./... && go vet ./... (no remaining in-loop DeleteStream calls)
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-07-09
status: complete
---

# Phase 2 Plan 2: Parallel Audio Version Processing Summary

**Audio version parallelization using golang.org/x/sync/errgroup — single-version path is sequential (unchanged), multi-version path parallelizes manifest fetching and license challenges for versions [1..N] with fail-fast cancellation, mutex-protected shared state, and deferred bulk stream cleanup**

## Performance

- **Duration:** 5 min
- **Started:** 2026-07-09T09:11:51Z
- **Completed:** 2026-07-09T09:17:42Z
- **Tasks:** 3
- **Files modified:** 5

## Accomplishments

- Restructured audio version processing in `episode.go` into three phases:
  - **Phase A:** Video + first audio (sequential, i==0) — unchanged behavior, uses `firstEpisode` directly
  - **Phase B:** Parallel audio for versions [1..N] via `errgroup.WithContext` — cache-first manifest, license challenge, audio download inside each goroutine; first error cancels all remaining work
  - **Phase C:** Deferred bulk stream cleanup — existing deferred closure iterates all `activeStreams` after parallel work completes
- Removed per-version `DeleteStream` calls from the loop body — all stream cleanup is handled by the existing deferred closure at function exit
- Added `sync.Mutex` to protect shared state (`activeStreams`, `tempFiles`, `audioTracks`) accessed from concurrent goroutines
- Added `golang.org/x/sync v0.22.0` as a direct dependency
- Added cache-first manifest access inside parallel goroutines using `GetCachedManifest`/`SetCachedManifest`
- All goroutines use `gctx` (errgroup-derived context), not parent `ctx`, ensuring sibling error propagation
- Added `NewTestClient` exported helper to `api` package for cross-package test client construction
- Added 3 test functions covering single-version, multi-version, and unavailable-locale paths
- Full project builds, vets, and all tests pass with `-race`

## Task Commits

Each task was committed atomically:

1. **Task 1: Add golang.org/x/sync dependency** - `27924d8` (chore)
2. **Task 2: Parallelize audio version processing with errgroup** - `0e99a85` (feat)
3. **Task 3: Add parallel audio test functions** - `82bdeda` (test)

**Plan metadata:** (committed below as part of SUMMARY.md commit)

## Files Created/Modified

- `go.mod` — Added `golang.org/x/sync v0.22.0` (direct dependency for errgroup)
- `go.sum` — Updated with v0.22.0 integrity hashes
- `internal/download/episode.go` — Restructured audio loop into Phase A (sequential i==0), Phase B (errgroup parallel [1..N]), and Phase C (deferred cleanup). Removed per-version DeleteStream/delete(activeStreams) calls. Added `"sync"` and `"golang.org/x/sync/errgroup"` imports.
- `internal/download/episode_test.go` — Added `TestEpisodeSingleVersion`, `TestEpisodeParallelAudio`, `TestEpisodeParallelAudioZeroVersions`. Existing tests unchanged.
- `internal/api/client_helper.go` — Added `NewTestClient` exported helper for constructing minimal `*Client` without real HTTP calls, enabling downstream test packages to create test clients.

## Decisions Made

- **errgroup fan-out with fail-fast (D-05, D-06):** Using `golang.org/x/sync/errgroup` for versions [1..N]. If any goroutine errors, the remaining goroutines are cancelled via `gctx`. This matches D-05/D-06 from CONTEXT.md.
- **Mutex-protected shared state (T-02.2-01, T-02.2-02):** `activeStreams`, `tempFiles`, and `audioTracks` are protected by `sync.Mutex` inside goroutines. Main goroutine reads these only after `g.Wait()` returns. This mitigates the tampering threats identified in the plan's threat model.
- **gctx everywhere (RESEARCH.md pitfall 1):** Every blocking call inside `g.Go` uses `gctx` (the errgroup-derived context), never the parent `ctx`. This ensures that when one goroutine fails, the cancelled context propagates to all siblings via `ctx.Done()` checks in `FetchManifest`, `GetLicense`, `DownloadParts`, etc.
- **No errgroup for single version:** When `len(versions) == 1`, only Phase A runs (purely sequential). No errgroup is created. This is the common case for single-dub downloads.
- **NewTestClient helper:** Added to enable test construction of `*api.Client` without real credentials. The `Client` struct's fields are unexported, making mock construction impossible from other test packages without this helper.
- **Per-version DeleteStream removed:** Following D-12 and D-13, all stream cleanup is deferred to the existing closure at function exit, which iterates `activeStreams` after all work (sequential + parallel) completes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] golang.org/x/sync dependency removed by go mod tidy**
- **Found during:** Task 1 (Add dependency)
- **Issue:** `go mod tidy` removes unused dependencies. Since the import wasn't added until Task 2, running `go mod tidy` in Task 1 removed the freshly-added `golang.org/x/sync` dependency from go.mod.
- **Fix:** Re-ran `go get golang.org/x/sync@v0.22.0` without `go mod tidy` for Task 1, and let Task 2's import cause `go mod tidy` to retain it. After Task 2, `go mod tidy` keeps the dependency as a direct (non-indirect) require.
- **Files modified:** go.mod, go.sum
- **Verification:** go.mod now contains `golang.org/x/sync v0.22.0` as a direct dependency. `go mod verify` passes.
- **Committed in:** `0e99a85` (Task 2 commit, where import was added)

**2. [Rule 3 - Blocking] Cannot construct test client from download package**
- **Found during:** Task 3 (Add tests)
- **Issue:** `api.Client` struct fields are all unexported, making it impossible to construct a minimal client from the `download` package's tests without either using `api.New()` (requires real `etp_rt` and makes real HTTP calls) or passing nil (panics at `c.url()`).
- **Fix:** Added `NewTestClient` exported helper to the `api` package that constructs a minimal `*Client` with configurable `httpClient`, `baseURL`, and `token`. Marked with doc comment noting this is a test helper.
- **Files modified:** internal/api/client_helper.go (created)
- **Verification:** Tests compile, run, and pass. All existing tests unchanged.
- **Committed in:** `82bdeda` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 bug, 1 blocking)
**Impact on plan:** Both auto-fixes necessary for correctness and testability. No scope creep.

## Issues Encountered

- `go mod tidy` removes unused dependencies — standard Go behavior required adjusting the task sequence to add import before tidying
- `api.Client` has only unexported fields, preventing mock construction from external packages — resolved via `NewTestClient` helper

## Next Phase Readiness

- PERF-06 complete: parallel audio version processing with errgroup, mutex-protected shared state, deferred cleanup
- Single-version path (most common case) unchanged — no errgroup overhead
- Full parallel integration test requires mock HTTP server infrastructure (planned for Phase 5)
- Next plan in Phase 2 is complete — ready for Phase 3 (Usability)

## Self-Check: PASSED

- [x] `go build ./...` — passes
- [x] `go vet ./...` — passes
- [x] `go test ./internal/download/... -race -count=1` — 6 tests pass (3 existing, 3 new)
- [x] `go test ./internal/media/... -race -count=1` — 20 tests pass
- [x] `go test ./... -race -count=1` — all packages pass
- [x] `go mod verify` — passed
- [x] go.mod contains `golang.org/x/sync v0.22.0` as direct dependency
- [x] `27924d8` — Task 1: chore(02-02): add golang.org/x/sync v0.22.0 dependency
- [x] `0e99a85` — Task 2: feat(02-02): parallelize audio version processing with errgroup
- [x] `82bdeda` — Task 3: test(02-02): add parallel audio test functions
- [x] SUMMARY.md created

---

*Phase: 02-performance-caching-parallelism*
*Completed: 2026-07-09*
