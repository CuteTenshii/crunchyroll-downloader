---
phase: 04-user-experience-progress-output
plan: 01
subsystem: output/download
tags: [output.global, per-episode-result, season-summary, error-accumulation]

# Dependency graph
requires:
  - phase: 04-user-experience-progress-output
    plan: 03
    provides: output.Global interface and Init() in internal/output package
provides:
  - Season download show per-episode result tracking with ✓/✗ markers
  - Season summary at end with total successes and failures
  - All fmt.Printf replaced with output.Global in episode.go and season.go
  - totalEpisodes parameter plumbed through Episode(), episodeDownloader, and all call sites
affects: [phase 05 (testing), any future output consumers]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - output.Global.Info/Warn/Error for all user-facing output
    - episodeError struct for structured error accumulation
    - Season() prints per-episode result line from runSeason() not Episode()
    - formatFileSize and formatDuration helper functions for human-readable output

key-files:
  created: []
  modified:
    - internal/download/episode.go - 11 fmt.Printf replaced, totalEpisodes param added, per-episode result line with ✓
    - internal/download/season.go - 2 fmt.Printf replaced, episodeError struct, season summary (success/failure)
    - internal/download/episode_test.go - Updated call sites with totalEpisodes=1
    - internal/download/season_test.go - Updated episodeDownloader func signature
    - main.go - Single-episode download call passes totalEpisodes=1

key-decisions:
  - "Episode() prints per-episode start line with [N/M] format using output.Global.Info"
  - "Episode() prints the success result line (✓ green) before return nil"
  - "Errors are NOT printed in Episode() — returned to Season() for inline display (✗ red)"
  - "totalEpisodes int parameter added to Episode() and episodeDownloader type"
  - "episodeError struct stores number, title, and error for structured error accumulation"
  - "SeasonError wraps first failure via %w to maintain errors.Is compatibility"
  - "Single-episode downloads pass totalEpisodes=1 (shows [Episode 1/1])"
  - "fmt import kept in episode.go (needed for fmt.Errorf, fmt.Sprintf in helpers)"

requirements-completed: [UX-02, UX-06]

# Coverage metadata
coverage:
  - id: D1
    description: "output.Global wired in episode.go — 11 fmt.Printf/fmt.Println calls replaced"
    requirement: UX-02
    verification:
      - kind: unit
        ref: "deviation via grep: rg 'fmt\\.(Printf|Println)' internal/download/episode.go returns 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "Per-episode start line [Episode N/M] and success result line with green ✓"
    requirement: UX-02
    verification:
      - kind: unit
        ref: "internal/download/episode_test.go passes (TestEpisodeSingleVersion, TestEpisodeParallelAudio)"
        status: pass
    human_judgment: false
  - id: D3
    description: "output.Global wired in season.go — 2 fmt.Printf calls replaced, no remaining raw print calls"
    requirement: UX-06
    verification:
      - kind: unit
        ref: "deviation via grep: rg 'fmt\\.(Printf|Println)' internal/download/season.go returns 0"
        status: pass
    human_judgment: false
  - id: D4
    description: "Per-episode error tracking with episodeError struct (number, title, err)"
    requirement: UX-06
    verification:
      - kind: unit
        ref: "grep for 'type episodeError struct' in internal/download/season.go"
        status: pass
    human_judgment: false
  - id: D5
    description: "Season success/failure summary with per-episode error listing per D-08"
    requirement: UX-06
    verification:
      - kind: unit
        ref: "internal/download/season_test.go passes (TestRunSeasonContinuesAfterEpisodeFailure)"
        status: pass
    human_judgment: false
  - id: D6
    description: "totalEpisodes parameter plumbed through Episode(), episodeDownloader, and all call sites"
    requirement: UX-02
    verification:
      - kind: unit
        ref: "grep for 'totalEpisodes int' in episode.go and season.go"
        status: pass
    human_judgment: false

# Metrics
duration: 14min
completed: 2026-07-10
status: complete
---

# Phase 4 Plan 1: Per-Episode Result Tracking & Season Summary Summary

**13 fmt.Printf calls replaced with output.Global in episode.go and season.go, per-episode result tracking with ✓/✗ markers, and season summary with error accumulation per D-06/D-07/D-08**

## Performance

- **Duration:** 14 min
- **Started:** 2026-07-10T01:45:00Z
- **Completed:** 2026-07-10T01:59:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- All 11 `fmt.Printf`/`fmt.Println` calls in `episode.go` replaced with `output.Global.Info`/`.Warn`/`.Debug`
- All 2 `fmt.Printf` calls in `season.go` replaced with `output.Global.Info`/`.Warn`/`.Error`
- `Episode()` now accepts `totalEpisodes int` parameter for `[Episode N/M]` formatting
- Per-episode start line: `[Episode N/M] Title (SxxExx) ...` via `output.Global.Info`
- Per-episode success result line: `[Episode N/M] Title — ✓ size, duration` (green ✓)
- Per-episode error result line: `[Episode N/M] Title — ✗ error` (red ✗, printed by Season)
- `episodeError` struct for structured error accumulation (number, title, error)
- Season success summary: `Season N download complete. All M episodes successful.` (green)
- Season failure summary: `Season N download complete. X of M episode(s) failed:` with indented per-episode error listing
- `formatFileSize` and `formatDuration` helper functions for human-readable output
- `SeasonError` wraps first failure via `%w` for `errors.Is` compatibility
- All tests pass (8 total), `go build ./...` and `go vet ./...` pass
- Single-episode downloads pass `totalEpisodes=1`, showing `[Episode 1/1]` format

## Task Commits

Each task was committed atomically:

1. **Task 1: Replace fmt.Printf in episode.go with output.Global, add per-episode result line** - `072d075` (feat)
2. **Task 2: Replace fmt.Printf in season.go, add per-episode error tracking and season summary** - `751965d` (feat)

## Files Created/Modified
- `internal/download/episode.go` - 64 insertions, 28 deletions — 11 fmt.Printf replaced, totalEpisodes param, per-episode result line, formatFileSize/formatDuration helpers
- `internal/download/season.go` - 59 insertions, 24 deletions — 2 fmt.Printf replaced, episodeError struct, season summary
- `internal/download/episode_test.go` - Updated call sites with totalEpisodes parameter
- `internal/download/season_test.go` - Updated episodeDownloader func literal signature
- `main.go` - Single-episode download call passes totalEpisodes=1

## Decisions Made
- **totalEpisodes parameter:** Added to `Episode()` and `episodeDownloader` type to enable `[Episode N/M]` formatting. Single-episode downloads receive `totalEpisodes=1`.
- **Result line placement:** `Episode()` prints the per-episode success result line (✓ green). Error result lines (✗ red) are printed by `Season()` to avoid code scatter across dozens of error return points in `Episode()`.
- **Error wrapping:** `SeasonError` uses `fmt.Errorf("...: %w", failures[0].Err)` to maintain `errors.Is` chain compatibility with the original failure. The combined error message uses `formatFailedList` for human-readable text.
- **fmt kept in imports:** `fmt` is still needed in `episode.go` for `fmt.Errorf`, `fmt.Sprintf` in output filename construction, and the new `formatFileSize`/`formatDuration` helpers.
- **errors package removed:** `season.go` no longer needs `errors.Join` since the new error handling uses `episodeError` struct and `formatFailedList`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Updated main.go single-episode Episode() call site**
- **Found during:** Task 2 (season.go changes)
- **Issue:** `main.go` line 146 calls `download.Episode()` directly and lacked the new `totalEpisodes int` parameter — build would fail
- **Fix:** Added `totalEpisodes=1` as the last argument
- **Files modified:** `main.go`
- **Verification:** `go build ./...` compiles successfully
- **Committed in:** `072d075` (Task 1 commit)

**2. [Rule 3 - Blocking] Updated episode_test.go and season_test.go call sites**
- **Found during:** Task 2 (season.go changes)
- **Issue:** Both test files had direct calls to `Episode()` and `runSeason()` with the old function signatures — build and vet would fail
- **Fix:** Added `totalEpisodes int` parameter (1 for isolated Episode calls) and updated the `episodeDownloader` function literal in `season_test.go`
- **Files modified:** `internal/download/episode_test.go`, `internal/download/season_test.go`
- **Verification:** `go test ./internal/download/...` passes (8/8)
- **Committed in:** `072d075` (Task 1), `751965d` (Task 2)

**3. [Rule 1 - Bug] Fixed SeasonError wrapping for errors.Is compatibility**
- **Found during:** Task 2 verification phase
- **Issue:** Replaced `errors.Join(failures...)` with `fmt.Errorf("season N: ...")` without `%w` — `errors.Is(err, firstErr)` in test failed
- **Fix:** Changed `fmt.Errorf` to include `%w` wrapping `failures[0].Err`: `fmt.Errorf("season %d: ...: %w", ..., failures[0].Err)`
- **Files modified:** `internal/download/season.go`
- **Verification:** `TestRunSeasonContinuesAfterEpisodeFailure` passes
- **Committed in:** `751965d` (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (2 blocking issues, 1 bug)
**Impact on plan:** All fixes necessary for compilation and correctness. No scope creep.

## Issues Encountered
- The edit tool matched the wrong `audioSet := manifest.Period[0].AdaptationSets[1]` block when replacing `fmt.Printf("Downloading %s audio...\n", ...)` because both the sequential and parallel blocks had identical surrounding code — required fix-up edits to correct indentation.
- `output.Global.Info()` prints `\033[K` (clear line) before the message and `\n` after via `fmt.Fprintln` — this interacts naturally with `\r` progress lines.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Ready for Plan 4.2 (segment progress with speed/ETA and main.go output migration)
- `episode.go` and `season.go` now use only `output.Global` for output — no raw `fmt.Printf` remains
- 13 of 43 total `fmt.Printf` calls replaced (cumulative: 28 from Plan 4.3 + 13 from Plan 4.1 = 41 of 43)

## Self-Check: PASSED

- [x] SUMMARY.md created at `.planning/phases/04-user-experience-progress-output/04-01-SUMMARY.md`
- [x] Task 1 committed: `072d075`
- [x] Task 2 committed: `751965d`
- [x] Summary metadata committed: `d5eb414`
- [x] `go build ./...` passes
- [x] `go vet ./...` passes
- [x] `go test ./internal/download/...` passes (8/8)

---
*Phase: 04-user-experience-progress-output*
*Completed: 2026-07-10*
