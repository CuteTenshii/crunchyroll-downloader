---
phase: 01-foundation-error-handling-http-memory
plan: 01-04
subsystem: drm
tags: [go, widevine, env, caching, tests]
requires:
  - phase: 01-foundation-error-handling-http-memory
    provides: "01-02 established context-aware license request flow that uses DRM device loading during license acquisition."
provides:
  - Widevine device loading is cached once per process and reused for license requests.
  - Device discovery uses explicit environment or `.env` paths instead of scanning the current directory.
  - `.wvd` device configuration takes precedence over raw client/key paths when both are configured.
  - Raw `client_id.bin` and `private_key.pem` paths are accepted only as a complete pair.
  - Missing or incomplete device configuration returns one clear error naming accepted formats.
affects: [drm, api, license, phase-01]
tech-stack:
  added: []
  patterns:
    - "Process-lifetime cache uses sync.Once around the Widevine device loader."
    - "Device path configuration is resolved from environment variables first, then `.env`."
    - "DRM tests reset package-level cache state through test helpers."
key-files:
  created:
    - internal/drm/drm_test.go
  modified:
    - internal/drm/drm.go
    - .planning/STATE.md
    - .planning/ROADMAP.md
    - .planning/REQUIREMENTS.md
key-decisions:
  - "Used `WIDEVINE_DEVICE_PATH`, `WIDEVINE_CLIENT_ID_PATH`, and `WIDEVINE_PRIVATE_KEY_PATH` as the explicit device path contract for this phase."
  - "Environment variables take precedence over `.env` values, and `.wvd` configuration takes precedence over raw pair configuration."
  - "Removed current-directory scanning instead of preserving it as a fallback, making missing or incomplete configuration fail fast."
patterns-established:
  - "Configuration discovery returns structured config before opening device files."
  - "All missing-device paths share the same user-facing error text."
requirements-completed: [PERF-05, QOL-13]
coverage:
  - id: D1
    description: "Widevine device loading is cached once per process and reused for repeated license requests."
    requirement: PERF-05
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestGetWidevineDeviceCachesLoader"
        status: pass
      - kind: other
        ref: "command: GOCACHE=/tmp/go-build go test ./internal/drm"
        status: pass
    human_judgment: false
  - id: D2
    description: "Device discovery loads explicit paths from environment or `.env`, with `.wvd` taking precedence over raw pair values."
    requirement: PERF-05
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigPrefersWVDFromDotEnv"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestLoadWidevineDeviceDoesNotFallbackWhenWVDConfigured"
        status: pass
    human_judgment: false
  - id: D3
    description: "Raw Widevine client and private key paths work only when both are configured together."
    requirement: QOL-13
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigRequiresRawPairTogether"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigAcceptsRawPair"
        status: pass
    human_judgment: false
  - id: D4
    description: "Missing device configuration returns one explicit accepted-formats error and no longer silently ignores `os.ReadDir` failures."
    requirement: QOL-13
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigMissingDeviceError"
        status: pass
      - kind: other
        ref: "command: rg \"os\\.ReadDir\" internal/drm"
        status: pass
    human_judgment: false
duration: 9 min
completed: 2026-07-08
status: complete
---

# Phase 01 Plan 01-04: Widevine Device Discovery Summary

**Widevine device loading now uses a process-lifetime cache with deterministic env and `.env` path discovery instead of repeated current-directory scans.**

## Performance

- **Duration:** 9 min
- **Started:** 2026-07-08T22:54:00Z
- **Completed:** 2026-07-08T23:02:59Z
- **Tasks:** 3 completed
- **Files modified:** 6

## Accomplishments

- Added `sync.Once` caching around the Widevine device loader so repeated license requests reuse the same loaded device.
- Replaced current-directory scanning with explicit `WIDEVINE_DEVICE_PATH`, `WIDEVINE_CLIENT_ID_PATH`, and `WIDEVINE_PRIVATE_KEY_PATH` lookup from environment variables or `.env`.
- Made `.wvd` configuration deterministic: it wins when raw client/key paths are also configured and does not silently fall back on failure.
- Enforced raw `client_id.bin` and `private_key.pem` path configuration as an all-or-nothing pair.
- Added DRM tests for cache reuse, `.env` loading, `.wvd` precedence, raw-pair completeness, and missing-device errors.

## Task Commits

Each task was committed atomically:

1. **Task 1: Process-lifetime Widevine loader cache** - `c1eb2e3` (feat)
2. **Task 2: Explicit env and `.env` device discovery** - `c9e6bb6` (feat)
3. **Task 3: Widevine discovery tests** - `1cbfc74` (test)

**Task-scope fix:** `c9d7c72` (fix) aligned the license fallback missing-device error with the new accepted configuration formats.

**Plan metadata:** committed separately with this summary.

## Files Created/Modified

- `internal/drm/drm.go` - Adds cached Widevine device loading, explicit env and `.env` discovery, deterministic precedence, raw-pair validation, and shared missing-device error text.
- `internal/drm/drm_test.go` - Covers cache reuse, `.wvd` precedence, raw-pair all-or-nothing behavior, and missing configuration errors.
- `.planning/STATE.md` - Records 01-04 completion and 4/5 phase progress.
- `.planning/ROADMAP.md` - Marks the Phase 1 Widevine device plan complete.
- `.planning/REQUIREMENTS.md` - Marks PERF-05 and QOL-13 complete.
- `.planning/phases/01-foundation-error-handling-http-memory/01-04-SUMMARY.md` - This summary.

## Decisions Made

- Used `WIDEVINE_DEVICE_PATH` for packaged `.wvd` files and `WIDEVINE_CLIENT_ID_PATH` plus `WIDEVINE_PRIVATE_KEY_PATH` for raw device pairs.
- Let real environment variables override `.env` values, matching common configuration precedence.
- Removed `os.ReadDir(".")` scanning entirely so missing device setup fails with one explicit accepted-formats error.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Unified missing-device error text across discovery and license paths**
- **Found during:** Final plan verification
- **Issue:** `GetLicense` still had an old fallback error that described current-directory `.wvd` and raw filenames after device discovery moved to env and `.env` paths.
- **Fix:** Added a shared `missingWidevineDeviceError()` helper and reused it in discovery and license fallback paths.
- **Files modified:** `internal/drm/drm.go`
- **Verification:** `GOCACHE=/tmp/go-build go test ./internal/drm` passed.
- **Committed in:** `c9d7c72`

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** The fix kept user-facing errors consistent with the new explicit configuration contract. No extra feature scope was added.

## Issues Encountered

- The default Go build cache path is read-only in the sandbox. Verification used `GOCACHE=/tmp/go-build`, matching prior Phase 1 plans.
- Git metadata writes are restricted by the sandbox, so staging and commits required approved escalation. Commits still ran normally with hooks; `--no-verify` was not used.

## Known Stubs

None. Stub scan found no TODO/FIXME placeholders or UI-facing empty mock data in files changed by this plan.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: file-access | `internal/drm/drm.go` | Device file paths now come from environment or `.env`; discovery is explicit, fail-fast, and covered by tests for missing and incomplete configuration. |

## Verification

- `GOCACHE=/tmp/go-build go test ./internal/drm` - passed
- `GOCACHE=/tmp/go-build go test ./...` - passed
- `rg "os\\.ReadDir" internal/drm` - no matches
- `.wvd` precedence verified by `TestDiscoverWidevineDeviceConfigPrefersWVDFromDotEnv` and `TestLoadWidevineDeviceDoesNotFallbackWhenWVDConfigured`.
- Raw pair all-or-nothing behavior verified by `TestDiscoverWidevineDeviceConfigRequiresRawPairTogether` and `TestDiscoverWidevineDeviceConfigAcceptsRawPair`.
- Missing config failure verified by `TestDiscoverWidevineDeviceConfigMissingDeviceError`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for the next Phase 1 plan. License acquisition now reuses one cached Widevine device and no longer performs repeated current-directory scans, which prepares the DRM path for later concurrent license work.

## Self-Check: PASSED

- Created files exist: `internal/drm/drm_test.go` and this summary.
- Modified files exist: `internal/drm/drm.go`, `.planning/STATE.md`, `.planning/ROADMAP.md`, and `.planning/REQUIREMENTS.md`.
- Task commits exist: `c1eb2e3`, `c9e6bb6`, `1cbfc74`, `c9d7c72`.
- Required verification passed with `GOCACHE=/tmp/go-build go test ./internal/drm`.
- Plan-level verification passed with `GOCACHE=/tmp/go-build go test ./...`.

---
*Phase: 01-foundation-error-handling-http-memory*
*Completed: 2026-07-08*
