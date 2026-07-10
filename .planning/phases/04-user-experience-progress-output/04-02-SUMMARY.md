---
phase: 04-user-experience-progress-output
plan: 02
subsystem: output
tags: [download-speed, eta, progress, SpeedTracker, rolling-window]

# Dependency graph
requires:
  - phase: 04-user-experience-progress-output
    plan: 03
    provides: output package with {RecordBytes, SpeedBps, ETASeconds} global functions
  - phase: 04-user-experience-progress-output
    plan: 01
    provides: episode.go fmt.Printf replacements and stream label integration points
provides:
  - Speed/ETA display on segment progress line (UX-03)
  - Throttled progress updates at ~1 Hz via atomic timestamp
  - formatSpeed / formatETAShort helper functions for human-readable formatting
  - streamLabel parameter on DownloadParts for per-stream progress context
affects:
  - Internal: segment.go progress display
  - Internal: episode.go callers of DownloadParts
  - Internal: segment_test.go test fixture

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Throttled progress via atomic nanosecond timestamp compare
    - Per-stream labeling via function parameter
    - ETA estimation using average segment size × remaining segments / rolling speed

key-files:
  created: []
  modified:
    - internal/media/segment.go — SpeedTracker integration, throttle, speed/ETA formatting
    - internal/download/episode.go — streamLabel parameter passed to DownloadParts calls
    - internal/media/segment_test.go — streamLabel parameter added to test call
    - internal/output/output_test.go — SpeedTracker global function tests

key-decisions:
  - "streamLabel string parameter added to DownloadParts (Option A) — cleaner than caller-side labeling"
  - "ETA calculated from average segment size x remaining segments / rolling speed (no upfront filesize)"
  - "Progress throttled via atomic.Int64 nanosecond timestamp, not time.Ticker (simpler, no goroutine)"
  - "Two ... separators in progress line: Downloaded N/M (P%) ... speed, ETA ... streamLabel"

patterns-established:
  - "Package-level atomic throttle guard for ~1 Hz progress updates"
  - "Helper functions (formatSpeed, formatETAShort) as private package utilities"

requirements-completed: [UX-03]

duration: 1min
completed: 2026-07-10
status: complete
---

# Phase 4 Plan 2: Download Speed/ETA — Summary

**SpeedTracker wired into segment.go with throttled speed/ETA progress display at ~1 Hz**

## Performance

- **Duration:** 1 min
- **Started:** 2026-07-10T00:51:17Z
- **Completed:** 2026-07-10T00:52:43Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- `DownloadParts` now accepts a `streamLabel string` for per-stream context (e.g., "video", "en-US audio")
- Segment progress line now shows: `Downloaded 145/212 segments (68%) ... 12.4 MB/s, ETA 23s ... video`
- Progress updates throttled to ~1 per second using atomic nanosecond timestamp
- `output.RecordBytes` called per-segment to feed SpeedTracker rolling window
- ETA estimated via average segment size × remaining segments / rolling speed
- `formatSpeed` converts Bps to human-readable (B/s, KB/s, MB/s, GB/s)
- `formatETAShort` converts seconds to "Ns" or "Nm Ns" format
- `fmt.Printf` and `fmt.Println` completely removed from segment.go (0 remaining)
- All 3 callers in episode.go updated with appropriate stream labels
- Test caller in segment_test.go updated with "test" stream label
- 3 new SpeedTracker tests: global accessors, multi-record, ETA clamping

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire SpeedTracker into segment.go** — `3ed61e9` (feat)
2. **Task 2: Add output package tests** — `993bd37` (test)

## Files Created/Modified
- `internal/media/segment.go` — Added output import, lastProgressNanos/totalBytesDownloaded atomics, throttled progress with speed/ETA, streamLabel parameter, formatSpeed/formatETAShort helpers, Debug completion message
- `internal/download/episode.go` — streamLabel passed to all 3 DownloadParts calls (audio, video, parallel audio)
- `internal/media/segment_test.go` — streamLabel="test" added to DownloadParts call
- `internal/output/output_test.go` — TestGlobalSpeedTrackerFunctions, TestSpeedTrackerWithMultipleRecords, TestSpeedTrackerETAClamping

## Decisions Made
- **streamLabel as parameter (Option A):** Added to `DownloadParts` rather than handling at call site. Cleaner design since the function controls its own progress output.
- **Atomic throttle**: Used `atomic.Int64` with nanosecond timestamps rather than `time.Ticker` — simpler, no extra goroutine, no channel management.
- **Average-segment-size ETA**: Since total filesize isn't known upfront, ETA estimates via average size of completed segments × remaining segments / rolling speed.
- **Debug for completion**: "Stream download complete: video" now goes to `output.Global.Debug` instead of `fmt.Println`.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered
- `atomic.Int64` used `.Load()/.Store()` methods instead of `atomic.LoadInt64/StoreInt64` functions (mixing old and new atomic APIs) — caught at build time and fixed in same commit. Initial compile error resolved by switching to method calls on `*atomic.Int64`.

## Verification Results
- `go build ./...` — passes ✓
- `go test ./internal/media/... -count=1` — passes ✓
- `go test ./internal/output/... -count=1 -run "SpeedTracker"` — passes ✓
- `go test ./... -count=1` — all packages pass ✓
- No `fmt.Printf` or `fmt.Println` remaining in segment.go ✓

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness
- Plan 04-2 completes the final piece of the fmt.Printf replacement project: all 43 original calls replaced across 8 files (28 in Plan 4.3 main/mux/api, 13 in Plan 4.1 episode/season, 2 in Plan 4.2 segment).
- Ready for verification and final phase review.

## Self-Check: PASSED
- `go build ./...`: PASS
- `"crunchyroll-downloader/internal/output"` imported in segment.go: PASS
- No `fmt.Printf`/`fmt.Println` in segment.go: PASS
- `streamLabel string` parameter on DownloadParts: PASS
- `lastProgressNanos` atomic throttle variable: PASS
- `formatSpeed` helper function: PASS
- `formatETAShort` helper function: PASS
- `github.com/iyear/gowidevine` import preserved: PASS
- feat commit (3ed61e9): PASS
- test commit (993bd37): PASS
- SUMMARY.md exists: PASS

---
*Phase: 04-user-experience-progress-output*
*Completed: 2026-07-10*
