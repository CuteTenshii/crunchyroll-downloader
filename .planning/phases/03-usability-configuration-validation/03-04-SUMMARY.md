---
phase: 03-usability-configuration-validation
plan: 04
subsystem: cli, download
tags: [regex, url-parsing, qol, code-quality, go-stdlib]

requires:
  - phase: 03-usability-configuration-validation
    plan: 02
    provides: url.Parse() in processURL, || operator (QOL-04/QOL-05), outputDir
  - phase: 03-usability-configuration-validation
    plan: 03
    provides: config resolution infrastructure, FFmpeg/Widevine startup flow

provides:
  - regex-based sanitizeFilename (QOL-07) — O(n) via regexp.MustCompile(_{2,})
  - parseLangs called once in main() after config resolution (QOL-08)
  - Pre-parsed lang slices passed as parameters to processURL

affects: []

tech-stack:
  added:
    - regexp (stdlib) — multiUnderscore var for underscore collapsing
  patterns:
    - Package-level regexp.MustCompile for compile-once, reuse-many
    - parseLangs called once after config resolution, slices passed as params

key-files:
  created: []
  modified:
    - internal/download/episode.go — regex sanitizeFilename, multiUnderscore var
    - internal/download/episode_test.go — TestSanitizeFilenameCollapsesMultiUnderscore
    - main.go — parseLangs moved to main(), processURL accepts []string slices
    - main_test.go — processURL calls updated with nil lang params

key-decisions:
  - "multiUnderscore var compiled once at package level — not per-call"
  - "parseLangs called once in main() after config resolution per D-22"
  - "Empty lang slices still get default values inside processURL (ja-JP for audio)"

requirements-completed: [QOL-04, QOL-05, QOL-07, QOL-08]

coverage:
  - id: D1
    description: "regex sanitizeFilename collapses multi-underscores (QOL-07)"
    requirement: QOL-07
    verification:
      - kind: unit
        ref: internal/download/episode_test.go#TestSanitizeFilenameCollapsesMultiUnderscore
        status: pass
    human_judgment: false
  - id: D2
    description: "processURL uses url.Parse() with correct || operator (QOL-04/QOL-05)"
    requirement: QOL-04
    verification:
      - kind: unit
        ref: main_test.go#TestProcessURLRejectsInvalidContentIDLength
        status: pass
      - kind: unit
        ref: main_test.go#TestProcessURLRejectsUnsupportedContentType
        status: pass
    human_judgment: false
  - id: D3
    description: "parseLangs called once in main() after config resolution (QOL-08)"
    requirement: QOL-08
    verification:
      - kind: unit
        ref: main_test.go#TestProcessURLRejectsInvalidContentIDLength (verifies processURL works with nil lang slices)
        status: pass
      - kind: unit
        ref: main_test.go#TestProcessURLRejectsUnsupportedContentType (verifies processURL works with nil lang slices)
        status: pass
    human_judgment: false

duration: 6 min
completed: 2026-07-09
status: complete
---

# Phase 3 Plan 4: Code Quality Fixes — regex sanitizeFilename, url.Parse(), parseLangs once

**Regex-based O(n) underscore collapsing in sanitizeFilename, url.Parse() and || operator already applied (Plan 3.2), parseLangs moved to main() with pre-parsed lang slices passed to processURL**

## Performance

- **Duration:** 6 min
- **Started:** 2026-07-09T11:11:00Z (approx)
- **Completed:** 2026-07-09T11:17:13Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- **QOL-07:** Replaced O(n²) `for strings.Contains(res, "__") { res = strings.ReplaceAll(...) }` loop with O(n) `multiUnderscore.ReplaceAllString(res, "_")` using package-level `regexp.MustCompile(_{2,})`
- **QOL-04/QOL-05:** Verified `processURL` already uses `url.Parse()` with correct `||` operator (applied in Plan 3.2)
- **QOL-08:** Moved `parseLangs` calls from inside `processURL` to `main()` after config resolution — called once, not per-URL in batch mode
- Pre-parsed `audioLangs`, `subsLangs` slices passed as parameters through `processURL`
- Table-driven test with 7 cases for `sanitizeFilename` collateralizing all edge cases
- All existing tests updated for new `processURL` signature

## Task Commits

Each task was committed atomically:

1. **Task 1a (RED): Add test for regex sanitizeFilename** - `a0ee350` (test)
2. **Task 1b (GREEN): Implement regex sanitizeFilename** - `a4e757b` (feat)
3. **Task 2: QOL-04/QOL-05 already in Plan 3.2** — verified, no changes needed
4. **Task 3: Move parseLangs to main() + update signature** - `27c0cd0` (feat)

## Files Created/Modified

- `internal/download/episode.go` — Added `"regexp"` import, `multiUnderscore` package var (`regexp.MustCompile(_{2,})`), replaced `for strings.Contains` loop with one-liner `multiUnderscore.ReplaceAllString`
- `internal/download/episode_test.go` — Added `TestSanitizeFilenameCollapsesMultiUnderscore` with 7 table-driven test cases
- `main.go` — `processURL` signature updated to accept `audioLangs, subsLangs []string`; `parseLangs` calls removed from inside `processURL`; `parseLangs` called once in `main()` after config resolution; pre-parsed slices passed to all `processURL` calls
- `main_test.go` — `processURL` test calls updated with `nil, nil` lang params

## Decisions Made

- `multiUnderscore` compiled once at package level using `regexp.MustCompile` — safe per threat model T-03.4-01 (bounded pattern, no ReDoS risk)
- Default language behavior preserved inside `processURL`: empty `audioLangs` → `["ja-JP"]`, empty `subsLangs` → no default sub locale
- QOL-04 and QOL-05 were already applied in Plan 3.2 — no further changes needed in this plan

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- Four code quality fixes (QOL-04, QOL-05, QOL-07, QOL-08) complete
- `sanitizeFilename` now uses O(n) regex instead of O(n²) loop
- `parseLangs` called once per invocation, not per-URL
- All Phase 3 plans complete — ready for phase verification

## Self-Check: PASSED

| Check | Status |
|-------|--------|
| `internal/download/episode.go` modified with regexp | ✓ |
| `internal/download/episode_test.go` modified with test | ✓ |
| `main.go` modified with parseLangs move | ✓ |
| `main_test.go` modified with new processURL calls | ✓ |
| 3 commits present (test → feat → feat) | ✓ |
| `go build .` passes | ✓ |
| `go test ./... -count=1` all OK (8 packages) | ✓ |
| `03-04-SUMMARY.md` created | ✓ |
