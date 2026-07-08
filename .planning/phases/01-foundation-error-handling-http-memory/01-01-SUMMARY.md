---
phase: 01-foundation-error-handling-http-memory
plan: 01-01
subsystem: download
tags: [go, error-handling, mux, ffmpeg, tests]
requires: []
provides:
  - Season download loop continues after individual episode failures and returns aggregate errors.
  - Episode orchestration propagates mux failures as errors.
  - FFmpeg mux failures return errors and remove partial output.
  - Mux cleanup failures are warnings when muxing succeeds.
affects: [download, mux, cli, phase-01]
tech-stack:
  added: []
  patterns:
    - "Injected episode downloader helper for season-loop tests"
    - "Injectable ffmpeg command runner for mux tests"
key-files:
  created:
    - internal/download/episode_test.go
    - internal/download/season_test.go
    - internal/mux/mux_test.go
  modified:
    - main.go
    - internal/download/episode.go
    - internal/download/season.go
    - internal/mux/mux.go
key-decisions:
  - "Season returns a SeasonError summary after attempting every episode, so batch behavior continues while callers can detect partial failure."
  - "Mux cleanup warnings are printed but do not change a successful mux result into a failure."
patterns-established:
  - "Return short user-facing errors instead of panicking in download and mux paths."
  - "Use test injection seams for process execution and episode orchestration."
requirements-completed: [UX-01, UX-05, QOL-10, QOL-11]
coverage:
  - id: D1
    description: "Episode failures return clear errors instead of crashing."
    requirement: UX-01
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go#TestEpisodeReturnsErrorForUnavailableAudioLocale"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/download ./internal/mux"
        status: pass
    human_judgment: false
  - id: D2
    description: "Season downloads continue after one episode fails and return an aggregate error."
    requirement: UX-05
    verification:
      - kind: unit
        ref: "internal/download/season_test.go#TestRunSeasonContinuesAfterEpisodeFailure"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/download ./internal/mux"
        status: pass
    human_judgment: false
  - id: D3
    description: "FFmpeg mux failures return errors and remove partial output."
    requirement: QOL-10
    verification:
      - kind: unit
        ref: "internal/mux/mux_test.go#TestMergeEverythingReturnsErrorAndRemovesPartialOutputOnFFmpegFailure"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/download ./internal/mux"
        status: pass
    human_judgment: false
  - id: D4
    description: "Mux cleanup failures are warnings and do not fail successful muxes."
    requirement: QOL-11
    verification:
      - kind: unit
        ref: "internal/mux/mux_test.go#TestMergeEverythingWarnsButSucceedsWhenCleanupFails"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/download ./internal/mux"
        status: pass
    human_judgment: false
duration: 7 min
completed: 2026-07-08
status: complete
---

# Phase 01 Plan 01-01: Foundation Error Handling Summary

**Download and mux paths now return explicit errors, keep season batches moving after episode failures, and warn on non-critical cleanup issues.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-08T22:16:00Z
- **Completed:** 2026-07-08T22:23:21Z
- **Tasks:** 3 completed
- **Files modified:** 11

## Accomplishments

- Season downloads now attempt every episode and return a `SeasonError` summary when one or more episodes fail.
- Episode orchestration now propagates mux failures instead of allowing `MergeEverything` to terminate the process.
- `MergeEverything` returns ffmpeg errors, removes partial output after mux failure, and prints cleanup warnings without failing successful muxes.
- Focused tests cover episode error returns, season continuation, mux failure cleanup, and cleanup-warning semantics.

## Task Commits

Each task was committed atomically:

1. **Task 1: Season continuation and aggregate errors** - `3c42c9e` (feat)
2. **Task 2: Mux error returns and cleanup warnings** - `9f63958` (fix)
3. **Task 3: Error contract test coverage** - `b41ac01` (test)

**Plan metadata:** committed separately with this summary.

## Files Created/Modified

- `main.go` - Reports returned season summary errors at CLI call sites.
- `internal/download/episode.go` - Propagates mux failures as episode errors.
- `internal/download/season.go` - Adds `SeasonError`, injectable season runner, continuation after per-episode failures, and aggregate returns.
- `internal/mux/mux.go` - Returns ffmpeg errors, removes partial output on failure, and warns on temp cleanup failures after success.
- `internal/download/episode_test.go` - Verifies episode-level unavailable-locale errors return cleanly.
- `internal/download/season_test.go` - Verifies the season loop continues after a failed episode and returns an aggregate error.
- `internal/mux/mux_test.go` - Verifies ffmpeg failure cleanup and cleanup-warning behavior.
- `.planning/STATE.md` - Records plan completion and next action.
- `.planning/ROADMAP.md` - Marks Phase 1 plan 1.1 complete.
- `.planning/REQUIREMENTS.md` - Marks UX-01, UX-05, QOL-10, and QOL-11 complete.
- `.planning/phases/01-foundation-error-handling-http-memory/01-01-SUMMARY.md` - This summary.

## Decisions Made

- `Season` returns a summary error after completing the loop instead of stopping on the first failed episode.
- The per-episode failure details remain logged in the season loop; callers receive a short aggregate summary.
- `MergeEverything` uses an injectable command runner so ffmpeg failure paths can be tested without invoking real ffmpeg.
- Cleanup failures after successful muxing are warnings only, preserving the successful download result.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The default Go build cache path was read-only in the sandbox. Verification used `GOCACHE=/tmp/go-build`, which keeps test artifacts in writable space.

## Known Stubs

None. Stub scan found only legitimate "not available" error text in episode validation paths.

## Threat Flags

None. The plan did not introduce new network endpoints, auth paths, file access trust boundaries, or schema changes.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/download ./internal/mux` - passed
- `rg panic internal/download internal/mux main.go` - no matches
- Season continuation verified by `TestRunSeasonContinuesAfterEpisodeFailure`
- Mux cleanup warning behavior verified by `TestMergeEverythingWarnsButSucceedsWhenCleanupFails`

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for `01-02-PLAN.md`. The download and mux error contracts are now explicit enough for the next memory/streaming changes to build on without panic-based control flow.

## Self-Check: PASSED

- Created files exist: `internal/download/episode_test.go`, `internal/download/season_test.go`, `internal/mux/mux_test.go`, and this summary.
- Task commits exist: `3c42c9e`, `9f63958`, `b41ac01`.
- Plan verification passed with `GOCACHE=/tmp/go-build go test ./internal/download ./internal/mux`.

---
*Phase: 01-foundation-error-handling-http-memory*
*Completed: 2026-07-08*
