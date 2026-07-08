---
phase: 01-foundation-error-handling-http-memory
plan: 01-03
subsystem: media
tags: [go, streaming, temp-files, workers, tests]
requires:
  - phase: 01-foundation-error-handling-http-memory
    provides: "01-02 established configured HTTP client and context-aware media request paths."
provides:
  - Configurable `--workers` segment download concurrency wired from CLI through season and episode downloads.
  - Segment assembly writes encrypted init and media payloads to a temp file before Widevine decrypt.
  - Media helper temp-file creation and malformed adaptation sets return errors instead of silent empty filenames.
  - Tests covering injected media clients, temp cleanup after segment failure, and malformed adaptation sets.
affects: [media, download, cli, phase-01]
tech-stack:
  added: []
  patterns:
    - "Segment download concurrency is caller-configured and defaults to 10 for backward compatibility."
    - "Encrypted segment payloads are assembled on disk and removed after decrypt."
    - "Media tests use fake HTTP doers instead of real network calls."
key-files:
  created:
    - internal/media/segment_test.go
    - internal/media/manifest_test.go
  modified:
    - main.go
    - internal/download/episode.go
    - internal/download/season.go
    - internal/download/episode_test.go
    - internal/download/season_test.go
    - internal/media/segment.go
    - internal/media/manifest.go
key-decisions:
  - "Kept the default segment worker count at 10 to preserve existing CLI behavior while adding `--workers` for explicit tuning."
  - "Used one encrypted temp assembly file per media track and removed it with defer after decrypt or failure."
  - "Kept media package decoupled from API internals by relying on the existing injected HTTP doer contract from 01-02."
patterns-established:
  - "Temp filename helpers return `(string, error)` and close handles immediately after creation."
  - "Malformed manifest helpers return nil/error values rather than falling through to empty filenames."
requirements-completed: [PERF-01, PERF-07, PERF-08, QOL-02, QOL-09, QOL-12]
coverage:
  - id: D1
    description: "Segment assembly streams encrypted init/media payloads through a temp file instead of concatenating a full in-memory payload before decrypt."
    requirement: PERF-01
    verification:
      - kind: other
        ref: "command: rg \"make\\(\\[\\]\\[\\]byte|append\\(parts|io\\.ReadAll\\(resp\\.Body\\).*DownloadParts\" internal/media/segment.go"
        status: pass
      - kind: unit
        ref: "internal/media/segment_test.go#TestDownloadPartsRemovesEncryptedTempOnSegmentFailure"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/media ./internal/download"
        status: pass
    human_judgment: false
  - id: D2
    description: "Segment and subtitle downloads use the configured injected HTTP client path."
    requirement: PERF-07
    verification:
      - kind: unit
        ref: "internal/media/segment_test.go#TestDownloadPartUsesInjectedClient"
        status: pass
      - kind: unit
        ref: "internal/media/segment_test.go#TestDownloadSubsUsesInjectedClientAndTempFile"
        status: pass
      - kind: other
        ref: "command: rg \"http\\.DefaultClient|http\\.Get\\(|http\\.Post\\(\" internal main.go"
        status: pass
    human_judgment: false
  - id: D3
    description: "`getFilename()` closes temp handles immediately after creation."
    requirement: PERF-08
    verification:
      - kind: unit
        ref: "internal/media/segment_test.go#TestGetFilenameReturnsCreateTempError"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/media"
        status: pass
    human_judgment: false
  - id: D4
    description: "`--workers` configures segment download concurrency end-to-end while keeping segment-count progress."
    requirement: QOL-02
    verification:
      - kind: unit
        ref: "internal/download/season_test.go#TestRunSeasonContinuesAfterEpisodeFailure"
        status: pass
      - kind: other
        ref: "command: rg \"workers\" main.go internal/download internal/media"
        status: pass
    human_judgment: false
  - id: D5
    description: "`os.CreateTemp` failures are checked and propagated."
    requirement: QOL-09
    verification:
      - kind: unit
        ref: "internal/media/segment_test.go#TestGetFilenameReturnsCreateTempError"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/media"
        status: pass
    human_judgment: false
  - id: D6
    description: "Empty or malformed adaptation sets return nil/error results instead of empty filenames or panics."
    requirement: QOL-12
    verification:
      - kind: unit
        ref: "internal/media/segment_test.go#TestGetFilenameRejectsEmptyAdaptationSet"
        status: pass
      - kind: unit
        ref: "internal/media/manifest_test.go#TestGetBaseUrlRejectsEmptyAdaptationSet"
        status: pass
      - kind: unit
        ref: "internal/media/manifest_test.go#TestGetBaseUrlSkipsMalformedRepresentation"
        status: pass
    human_judgment: false
duration: 15 min
completed: 2026-07-08
status: complete
---

# Phase 01 Plan 01-03: Streaming Segment Assembly Summary

**Segment downloads now use configurable concurrency and disk-backed encrypted assembly with tested temp-file and manifest edge-case handling.**

## Performance

- **Duration:** 15 min
- **Started:** 2026-07-08T22:38:10Z
- **Completed:** 2026-07-08T22:53:49Z
- **Tasks:** 3 completed
- **Files modified:** 12

## Accomplishments

- Added `--workers` and threaded it through episode and season orchestration into `DownloadParts`.
- Replaced final full-payload segment concatenation with encrypted temp-file assembly followed by disk-backed Widevine decrypt.
- Made temp filename creation return errors and close handles immediately, preventing `getFilename()` descriptor leaks.
- Guarded empty and malformed adaptation sets in media helpers.
- Added media tests for injected client usage, temp cleanup after segment failure, CreateTemp failures, and malformed adaptation sets.

## Task Commits

Each task was committed atomically:

1. **Task 1: Configurable segment workers** - `4b30b24` (feat)
2. **Task 2: Temp-file segment streaming** - `fe177b0` (perf)
3. **Task 3: Media temp/adaptation hardening and tests** - `075f497` (fix)

**Plan metadata:** committed separately with this summary.

## Files Created/Modified

- `main.go` - Adds `--workers` flag and passes it into episode/season downloads.
- `internal/download/episode.go` - Passes worker count into audio and video `DownloadParts` calls.
- `internal/download/season.go` - Threads worker count through season orchestration and injectable downloader helper.
- `internal/download/episode_test.go` - Updates episode test for worker-aware signature.
- `internal/download/season_test.go` - Verifies worker count reaches the injected episode downloader.
- `internal/media/segment.go` - Streams encrypted media assembly through temp files, checks temp errors, closes handles, and preserves segment-count progress.
- `internal/media/manifest.go` - Guards nil, empty, and malformed adaptation-set representations.
- `internal/media/segment_test.go` - Covers injected client usage, subtitle temp writing, CreateTemp errors, empty adaptation sets, and cleanup after segment failure.
- `internal/media/manifest_test.go` - Covers empty and malformed adaptation-set guards.
- `.planning/STATE.md` - Records 01-03 completion and 3/5 phase progress.
- `.planning/ROADMAP.md` - Marks the Phase 1 work covered by 01-03 complete.
- `.planning/REQUIREMENTS.md` - Marks PERF-01, PERF-08, QOL-02, QOL-09, and QOL-12 complete.

## Decisions Made

- Kept the default worker count at 10 to preserve existing behavior while making concurrency configurable.
- Used an encrypted temp assembly file rather than a giant byte slice before decrypting, keeping the decrypt call compatible with `DecryptMP4Auto`.
- Reused the injected HTTP doer contract from 01-02 instead of coupling `internal/media` to `internal/api`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Threaded workers through season orchestration**
- **Found during:** Task 1 (configurable worker count)
- **Issue:** The task file list omitted `internal/download/season.go`, but season downloads also invoke `Episode` and need the chosen worker budget.
- **Fix:** Updated `Season`, `runSeason`, and season tests to carry the worker count into the episode downloader.
- **Files modified:** `internal/download/season.go`, `internal/download/season_test.go`
- **Verification:** `GOCACHE=/tmp/go-build go test ./internal/media ./internal/download` passed, and the season test asserts the worker value.
- **Committed in:** `4b30b24`

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** The deviation was required for correct end-to-end `--workers` behavior. No unrelated scope was added.

## Issues Encountered

- TokenSave returned context for a different stale worktree, so local file reads and tests were used as the source of truth.
- The default Go build cache path is not writable in the sandbox. Verification used `GOCACHE=/tmp/go-build`.

## Known Stubs

None. Stub scan found only legitimate user-facing "not available" validation messages in episode locale checks.

## Threat Flags

None. This plan changed outbound media download behavior and temp-file handling but did not add new network endpoints, auth paths, schema changes, or new trust-boundary file inputs.

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/media ./internal/download` - passed
- `GOCACHE=/tmp/go-build go test ./...` - passed
- `rg "http\\.DefaultClient|http\\.Get\\(|http\\.Post\\(" internal main.go` - no matches
- `rg "make\\(\\[\\]\\[\\]byte|append\\(parts|io\\.ReadAll\\(resp\\.Body\\).*DownloadParts" internal/media/segment.go` - no matches
- Segment progress remains segment-based via `done.Add(1)` and `Downloaded %v of %v segments`.
- Temp cleanup after segment failure is covered by `TestDownloadPartsRemovesEncryptedTempOnSegmentFailure`.
- Malformed adaptation sets are covered by `TestGetFilenameRejectsEmptyAdaptationSet`, `TestGetBaseUrlRejectsEmptyAdaptationSet`, and `TestGetBaseUrlSkipsMalformedRepresentation`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for `01-04-PLAN.md`. Segment downloads now have bounded configurable concurrency, shared-client coverage, and disk-backed assembly, so follow-on Widevine and manifest performance work can build on lower memory pressure and stricter media helper errors.

## Self-Check: PASSED

- Created files exist: `internal/media/segment_test.go`, `internal/media/manifest_test.go`, and this summary.
- Task commits exist: `4b30b24`, `fe177b0`, `075f497`.
- Required verification passed with `GOCACHE=/tmp/go-build go test ./internal/media ./internal/download`.
- Plan-level verification passed with `GOCACHE=/tmp/go-build go test ./...`.

---
*Phase: 01-foundation-error-handling-http-memory*
*Completed: 2026-07-08*
