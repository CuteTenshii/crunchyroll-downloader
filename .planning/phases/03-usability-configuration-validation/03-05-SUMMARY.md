---
phase: 03-usability-configuration-validation
plan: 05
subsystem: auth
tags: [go, env-var, basic-auth]

# Dependency graph
requires:
  - phase: 01-foundation-error-handling-http-memory
    provides: API client structure with fetchAccessToken in auth.go
provides:
  - CRUNCHYROLL_CLIENT_AUTH env var override for hardcoded Basic Auth credential
  - getClientAuth() function with documented default constant
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - os.LookupEnv for env var fallback (same pattern as internal/drm/drm.go)
    - t.Setenv for env var test cleanup

key-files:
  created:
    - internal/api/auth_test.go
  modified:
    - internal/api/auth.go

key-decisions:
  - "CRUNCHYROLL_CLIENT_AUTH env var checked on every fetchAccessToken call (not cached at startup)"
  - "Empty env var treated as 'not provided' — falls back to compiled-in default"
  - "Same-package test (package api) for access to unexported getClientAuth() and defaultClientAuth"

patterns-established:
  - "Env var overrides use os.LookupEnv() called per-invocation, not cached at startup"

requirements-completed: [USAB-07]

# Coverage metadata
coverage:
  - id: D1
    description: "CRUNCHYROLL_CLIENT_AUTH env var override for Basic Auth credential in auth.go"
    requirement: USAB-07
    verification:
      - kind: unit
        ref: internal/api/auth_test.go#TestGetClientAuthReturnsDefault
        status: pass
      - kind: unit
        ref: internal/api/auth_test.go#TestGetClientAuthPrefersEnv
        status: pass
      - kind: unit
        ref: internal/api/auth_test.go#TestGetClientAuthEmptyEnvFallsBack
        status: pass
    human_judgment: false

duration: 1 min
completed: 2026-07-09
status: complete
---

# Phase 3 Plan 5: Env Var Auth Override

**CRUNCHYROLL_CLIENT_AUTH env var override for hardcoded Basic Auth credential with getClientAuth() function and documented default constant**

## Performance

- **Duration:** 1 min
- **Started:** 2026-07-09T10:53:18Z
- **Completed:** 2026-07-09T10:54:38Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Added `getClientAuth()` function that checks `CRUNCHYROLL_CLIENT_AUTH` env var on every call
- Added `defaultClientAuth` constant preserving the known public Crunchyroll client ID with documentation
- Replaced hardcoded Basic Auth string in `fetchAccessToken()` with `getClientAuth()` call
- Created test file with 4 tests covering default, env var override, empty env fallback, and constant value verification

## Task Commits

Each task was committed atomically:

1. **Task 1: Add getClientAuth() function and CRUNCHYROLL_CLIENT_AUTH env var support** - `78a5197` (feat)

## Files Created/Modified

- `internal/api/auth.go` - Added `getClientAuth()` function, `defaultClientAuth` constant, `"os"` import; replaced hardcoded auth string
- `internal/api/auth_test.go` - Tests for `getClientAuth()` with default, env override, and empty env fallback cases

## Decisions Made

- `CRUNCHYROLL_CLIENT_AUTH` env var is checked on every `fetchAccessToken()` call (per D-08), enabling credential rotation without restart
- Empty env var value is treated as "not provided" — falls back to compiled-in default
- Same-package test (`package api`) used for access to unexported `getClientAuth()` and `defaultClientAuth`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 03-05 complete. Ready for next plan in Phase 3 (Usability — Configuration & Validation).

## Self-Check: PASSED

| Check | Status |
|-------|--------|
| internal/api/auth_test.go created | ✓ |
| internal/api/auth.go modified | ✓ |
| Commit 78a5197 exists | ✓ |
| `go test ./internal/api/...` passes | ✓ |
| 03-05-SUMMARY.md created | ✓ |

---

*Phase: 03-usability-configuration-validation*
*Completed: 2026-07-09*
