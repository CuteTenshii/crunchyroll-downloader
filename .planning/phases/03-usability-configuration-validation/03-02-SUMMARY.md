---
phase: 03-usability-configuration-validation
plan: 02
subsystem: cli
tags: [output-dir, url-validation, cli-flags, go-stdlib]

requires:
  - phase: 03-usability-configuration-validation
    plan: 01
    provides: resolvedOutputDir from precedence chain, outputDir flag definition
provides:
  - --output-dir CLI flag wired through processURL → Season/runSeason → Episode
  - validateURL/validateAllURLs for batch URL validation using url.Parse()
  - validateOutputDir function for output directory existance check
  - QOL-04 fix: && → || in content ID length check
  - QOL-05 fix: url.Parse() replaces string-split URL parsing
affects:
  - 03-03 (will wire widevine-device into drm package)

tech-stack:
  added: []
  patterns:
    - url.Parse() for URL path parsing (QOL-05)
    - Pre-validation of all batch URLs before any download work (D-17)
    - outputDir parameter threading through download call chain

key-files:
  created: []
  modified:
    - main.go
    - main_test.go
    - internal/download/episode.go
    - internal/download/season.go
    - internal/download/episode_test.go
    - internal/download/season_test.go

key-decisions:
  - "Episode() and Season() use outputDir string param — empty string means CWD default"
  - "validateURL uses url.Parse() for correct path extraction (handles trailing slashes, query params)"
  - "validateAllURLs collects ALL invalid URLs upfront — does not stop at first error"
  - "Output dir validation extracted as validateOutputDir function for testability"
  - "Batch URLs validated upfront before any downloads start (D-17)"

patterns-established:
  - "Parameter threading: add new params at the end of existing function signatures before error return"
  - "Pre-validation pattern: validate all inputs upfront, fail fast with complete error list"

requirements-completed: [USAB-03, USAB-04]

coverage:
  - id: D1
    description: "--output-dir CLI flag wiring through main → Season → Episode"
    requirement: USAB-03
    verification:
      - kind: unit
        ref: "main_test.go#TestOutputDirMissingDirErrors"
        status: pass
      - kind: unit
        ref: "main_test.go#TestOutputDirEmptyIsValid"
        status: pass
      - kind: unit
        ref: "main_test.go#TestOutputDirFileIsNotValid"
        status: pass
      - kind: unit
        ref: "main_test.go#TestOutputDirValidDirPasses"
        status: pass
      - kind: unit
        ref: "internal/download/episode_test.go#TestOutputDirCreatesSeriesSubfolderInOutputDir"
        status: pass
    human_judgment: false
  - id: D2
    description: "validateURL/validateAllURLs for batch URL validation using url.Parse()"
    requirement: USAB-04
    verification:
      - kind: unit
        ref: "main_test.go#TestValidateURLValidWatchPath"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateURLValidSeriesPath"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateURLTooShort"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateURLTooLong"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateURLWrongContentType"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateURLTrailingSlash"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateURLWithQueryParams"
        status: pass
      - kind: unit
        ref: "main_test.go#TestValidateAllURLsReportsAll"
        status: pass
    human_judgment: false

duration: 5min
completed: 2026-07-09
status: complete
---

# Phase 3 Plan 2: --output-dir Flag and Batch URL Validation Summary

**--output-dir CLI flag wired through Season → Episode call chain, batch URL validation with url.Parse(), and comprehensive test coverage**

## Performance

- **Duration:** 5 min
- **Started:** 2026-07-09T10:55:30Z
- **Completed:** 2026-07-09T11:00:45Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- Added `outputDir string` parameter to `Episode()`, `Season()`, `runSeason()`, and `episodeDownloader` type
- Output path construction uses `filepath.Join(outputDir, seriesTitle)` when outputDir is set; existing CWD behavior when empty
- `validateOutputDir()` checks path exists and is a directory at startup (D-11 — no auto-create)
- `validateURL()` uses `url.Parse()` for correct URL path extraction (QOL-05) with proper `/watch/` or `/series/` + 9-14 char ID validation
- `validateAllURLs()` validates all batch URLs upfront, reporting ALL invalid entries (D-17)
- Fixed `&&` → `||` bug in content ID length check (QOL-04)
- Refactored `processURL()` to use `url.Parse()` instead of string splitting
- All existing tests updated for new `outputDir` parameter signatures

## Task Commits

Each task was committed atomically:

1. **Task 1: Add --output-dir through call chain** - `af7bc71` (feat)
2. **Task 2: Add batch URL validation** - `7d23a74` (feat)
3. **Task 3: Add output dir validation tests** - `65fad22` (test)

## Files Created/Modified

- `main.go` - validateURL, validateAllURLs, validateOutputDir, invalidURL struct, processURL refactored for url.Parse(), batch validation wiring, outputDir parameter
- `main_test.go` - 13 new tests: 8 validateURL/validateAllURLs tests + 4 output dir validation tests
- `internal/download/episode.go` - outputDir param, filepath.Join for output base
- `internal/download/season.go` - outputDir param through episodeDownloader/Season/runSeason
- `internal/download/episode_test.go` - Updated Episode() calls with outputDir, new TestOutputDirCreatesSeriesSubfolderInOutputDir
- `internal/download/season_test.go` - Updated runSeason() call with outputDir param

## Decisions Made

- Output dir validation extracted to `validateOutputDir` function for testability (returns error string instead of inline os.Exit)
- `invalidURL` defined as package-level struct for reuse in validation functions
- `processURL` parameter renamed to `rawURL` to avoid shadowing `net/url` package import
- All test calls use `""` for outputDir to preserve existing CWD behavior testing

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Initial `url` parameter name in `processURL` shadowed `net/url` package import — renamed to `rawURL`
- `season_test.go` existed with `runSeason` call that also needed `outputDir` parameter update

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `--output-dir` flag fully wired and validated
- Batch URL validation prevents invalid URLs from reaching the download loop
- `--widevine-device` flag defined but not yet wired (Plan 3.3)
- Next plan: 03-03 — Widevine device path + FFmpeg check

## Self-Check: PASSED

- `internal/download/episode.go`: FOUND ✓
- `internal/download/season.go`: FOUND ✓
- `internal/download/episode_test.go`: FOUND ✓
- `internal/download/season_test.go`: FOUND ✓
- `main.go`: FOUND ✓
- `main_test.go`: FOUND ✓
- Commits: 3 task commits present (2 feat, 1 test): af7bc71, 7d23a74, 65fad22 ✓
- `go build .`: PASS ✓
- `go build ./internal/download/...`: PASS ✓
- `go test ./internal/download/... . -count=1`: PASS ✓

---

*Phase: 03-usability-configuration-validation*
*Completed: 2026-07-09*
