---
phase: 02-performance-caching-parallelism
plan: 03
subsystem: media
tags: go, refactoring, mpd, manifest, bandwidth-matching

requires:
  - phase: 01-foundation-error-handling-http-memory
    provides: test infrastructure for manifest functions

provides:
  - GetVideoBaseUrl replacing GetBaseUrl(isVideoSet=true) for video height matching
  - GetAudioBaseUrl replacing GetBaseUrl(isVideoSet=false) with explicit switch/case bandwidth mapping
  - Comprehensive test coverage for both split functions including bandwidth thresholds and fallback

affects: []
tech-stack:
  added: []
  patterns:
    - Explicit switch/case for audio quality thresholds instead of string-manipulation fallthrough
    - Per-media-type function dispatch replacing boolean parameter polymorphism

key-files:
  created: []
  modified:
    - internal/media/manifest.go — GetBaseUrl removed, GetVideoBaseUrl and GetAudioBaseUrl added
    - internal/download/episode.go — callers updated to split functions
    - internal/media/manifest_test.go — tests migrated and expanded with bandwidth matching

key-decisions:
  - "GetAudioBaseUrl uses explicit switch/case per D-11 instead of threshold fallthrough from GetBaseUrl"
  - "Unrecognized audio quality (e.g. '999k') falls through to first available representation"
  - "GetVideoBaseUrl retains strings.ReplaceAll for p-suffix stripping (no behavior change)"
  - "strings import retained for strings.Contains in GetAudioBaseUrl and strings.ReplaceAll in GetVideoBaseUrl"

patterns-established:
  - "Audio bandwidth matching uses switch/case with literal bandwidth thresholds (192000, 128000, 96000)"
  - "Video height matching via strconv.ParseInt with p-suffix stripping"

requirements-completed:
  - QOL-03

coverage:
  - id: D1
    description: "GetVideoBaseUrl replaces GetBaseUrl(isVideoSet=true) — matches video representation by height, returns (BaseURL, ID)"
    requirement: QOL-03
    verification:
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetVideoBaseUrlMatchesHeight
        status: pass
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetBaseUrlRejectsEmptyAdaptationSet
        status: pass
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetBaseUrlSkipsMalformedRepresentation
        status: pass
    human_judgment: false
  - id: D2
    description: "GetAudioBaseUrl replaces GetBaseUrl(isVideoSet=false) with explicit switch/case for 192k, 128k, 96k — unknown quality falls through to first rep"
    requirement: QOL-03
    verification:
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetAudioBaseUrlBandwidth192k
        status: pass
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetAudioBaseUrlBandwidth128k
        status: pass
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetAudioBaseUrlBandwidth96k
        status: pass
      - kind: unit
        ref: internal/media/manifest_test.go#TestGetAudioBaseUrlFallback
        status: pass
    human_judgment: false
  - id: D3
    description: "episode.go callers updated — no remaining GetBaseUrl references in production code, full project compiles and vets cleanly"
    verification:
      - kind: unit
        ref: internal/download/episode_test.go#TestEpisodeReturnsErrorForUnavailableAudioLocale
        status: pass
      - kind: integration
        ref: go build ./... && go vet ./... (verified manually)
        status: pass
    human_judgment: false

duration: 3 min
completed: 2026-07-09
status: complete
---

# Phase 2: Performance, Caching & Parallelism — Plan 3 Summary

**Split `GetBaseUrl` by media type — removed the fragile `isVideoSet` boolean parameter, introduced `GetVideoBaseUrl` (height matching) and `GetAudioBaseUrl` (explicit switch/case bandwidth mapping per D-11)**

## Performance

- **Duration:** 3 min
- **Started:** 2026-07-09T09:05:38Z
- **Completed:** 2026-07-09T09:09:33Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Removed monolithic `GetBaseUrl(set, isVideoSet bool, quality)` with fragile boolean parameter
- Added `GetVideoBaseUrl(set, quality)` — matches video representation by height (unchanged logic)
- Added `GetAudioBaseUrl(set, quality)` — uses explicit `switch/case` for bandwidth thresholds per D-11, rejecting unknown quality gracefully (falls through to first available representation)
- Updated two call sites in `episode.go` to use the new split functions
- Migrated existing tests and added comprehensive bandwidth-matching tests for all three quality tiers (192k, 128k, 96k) plus fallback behavior
- Zero remaining `GetBaseUrl` references in production code
- Full project builds, vets, and all tests pass with `-race`

## Task Commits

Each task was committed atomically:

1. **Task 1: Split GetBaseUrl into GetVideoBaseUrl and GetAudioBaseUrl in manifest.go** - `b883bb9` (refactor)
2. **Task 2: Update GetBaseUrl callers in episode.go to use split functions** - `c584d85` (refactor)
3. **Task 3: Update existing tests and add bandwidth-matching tests for GetAudioBaseUrl** - `8aff1b2` (test)

## Files Created/Modified

- `internal/media/manifest.go` — Removed `GetBaseUrl`, added `GetVideoBaseUrl` and `GetAudioBaseUrl` with docs
- `internal/download/episode.go` — Updated both call sites to use the split functions
- `internal/media/manifest_test.go` — Migrated existing tests, added `TestGetVideoBaseUrlMatchesHeight` (3 subtests), `TestGetAudioBaseUrlBandwidth192k/128k/96k`, `TestGetAudioBaseUrlFallback` (2 subtests)

## Decisions Made

- **Explicit switch/case:** The old `GetBaseUrl` used string manipulation (`strings.ReplaceAll(quality, "k", "")`) with numeric comparisons. The new `GetAudioBaseUrl` uses explicit `switch/case` with each quality string mapped to its literal bandwidth threshold, per design decision D-11. This eliminates the fragile string-manipulation fallthrough pattern.
- **Unrecognized quality:** If an unknown quality string is provided for audio (e.g., "999k"), the `default` case in the switch falls through to the next representation. If no representation matches, the function returns the first available representation as a fallback.
- **Imports preserved:** Both `strconv` (for `strconv.ParseInt` in video matching) and `strings` (for `strings.Contains` in audio matching and `strings.ReplaceAll` in video matching) remain imported.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

All three QOL-03 subtasks complete: `GetBaseUrl` is removed, the two split functions are in place, and comprehensive tests cover bandwidth matching across all quality tiers. Ready for the next plan in Phase 2.

## Self-Check: PASSED

- All 4 commits exist (3 task commits + 1 docs commit)
- `GetBaseUrl` completely removed from production code
- `go build ./...` passes
- `go vet ./...` passes with no undefined references
- `go test ./internal/media/... -race -count=1` — all 20 tests pass
- `go test ./internal/download/... -race -count=1` — all 3 tests pass
- Test function names `TestGetBaseUrlRejectsEmptyAdaptationSet` and `TestGetBaseUrlSkipsMalformedRepresentation` retained as legacy names — they call `GetVideoBaseUrl`

---

*Phase: 02-performance-caching-parallelism*
*Completed: 2026-07-09*
