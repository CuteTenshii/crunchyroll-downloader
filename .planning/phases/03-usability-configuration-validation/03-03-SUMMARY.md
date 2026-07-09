---
phase: 03-usability-configuration-validation
plan: 03
subsystem: drm, cli
tags: ffmpeg, widevine, startup-validation, device-path, cli-flags
requires:
  - phase: 03-usability-configuration-validation
    plan: 02
    provides: config file support, output-dir flag, resolveString helper
provides:
  - FFmpeg startup validation via exec.LookPath + exec.Command
  - --widevine-device flag with auto-detection (WVD vs raw directory)
  - SetWidevinePath called before sync.Once for correct device loading
  - DetectDevicePath for path format classification
  - Removed .env file reading from drm package
affects: 03-04, 03-05
tech-stack:
  added: os/exec (FFmpeg check), path/filepath (DetectDevicePath)
  patterns: [startup validation before API client, SetWidevinePath before sync.Once, CLI flag > env var > config precedence]
key-files:
  created: []
  modified:
    - main.go — checkFFmpeg(), --widevine-device wiring, drm.SetWidevinePath call
    - internal/drm/drm.go — SetWidevinePath, DetectDevicePath, DevicePathFormat, removed readDotEnv
    - internal/drm/drm_test.go — DetectDevicePath tests, SetWidevinePath tests, removed .env tests
key-decisions:
  - "FFmpeg check runs after config resolution but before API client creation (D-18, D-19)"
  - "SetWidevinePath called before api.NewWithContext to ensure sync.Once uses correct path (Pitfall 3)"
  - "Widevine path auto-detection uses os.Stat: .wvd extension → FormatWVD, directory with client_id.bin+private_key.pem → FormatRawDir"
  - "Legacy env var names remain as direct os.LookupEnv fallbacks (D-15)"
  - ".env file reading removed entirely from drm.go (D-16)"
requirements-completed: [USAB-05, USAB-06]
coverage:
  - id: D1
    description: "FFmpeg checkFFmpeg function validates ffmpeg is on PATH and executable"
    requirement: USAB-05
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDetectDevicePath* (compilation verification)"
        status: pass
    human_judgment: true
    rationale: "checkFFmpeg can only be tested by removing ffmpeg from PATH, which is environment-dependent and best verified manually"
  - id: D2
    description: "SetWidevinePath, DetectDevicePath, DevicePathFormat constants in drm package"
    requirement: USAB-06
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestSetWidevinePathOverridesEnvVars"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestSetWidevinePathWVDOverridesEnvVars"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDetectDevicePathAcceptsWVD"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDetectDevicePathAcceptsRawDir"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDetectDevicePathRejectsMissingPath"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDetectDevicePathRejectsPlainFile"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDetectDevicePathRejectsEmptyDir"
        status: pass
    human_judgment: false
  - id: D3
    description: ".env file reading removed, legacy env var names retained as os.LookupEnv fallbacks"
    requirement: USAB-06
    verification:
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigPrefersWVD"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigEnvOverrideWithSetWidevinePath"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigAcceptsRawPair"
        status: pass
      - kind: unit
        ref: "internal/drm/drm_test.go#TestDiscoverWidevineDeviceConfigMissingDeviceError"
        status: pass
    human_judgment: false
duration: 4min
completed: 2026-07-09
status: complete
---

# Phase 3 Plan 3: FFmpeg Startup Validation + --widevine-device Flag

**FFmpeg startup check via exec.LookPath, --widevine-device flag with auto-detect (WVD/raw dir), SetWidevinePath integration, and complete removal of .env file reading from drm package**

## Performance

- **Duration:** 4 min
- **Started:** 2026-07-09T11:06:30Z (approx)
- **Completed:** 2026-07-09T11:09:59Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- `checkFFmpeg()` startup validation — hard error with actionable message if ffmpeg is missing (D-18, D-19)
- `--widevine-device` flag resolved through precedence: CLI flag > env var > config file > default
- `drm.SetWidevinePath()` called before `api.NewWithContext()` — guarantees `sync.Once` uses correct path (Pitfall 3)
- `DetectDevicePath()` with `DevicePathFormat` — auto-detects `.wvd` file vs directory with `client_id.bin` + `private_key.pem`
- `.env` file reading removed entirely — `readDotEnv()` and `envValue()` functions deleted
- Legacy env var names (`WIDEVINE_DEVICE_PATH`, `WIDEVINE_CLIENT_ID_PATH`, `WIDEVINE_PRIVATE_KEY_PATH`) remain as `os.LookupEnv` fallbacks
- 16 passing drm package tests (up from 7), including 5 DetectDevicePath tests + 2 SetWidevinePath tests
- Existing tests updated from `.env`-based to direct env var (`t.Setenv`) approach

## Task Commits

Each task was committed atomically:

1. **Task 2: Modify drm.go** — `25eabd1` (feat: add SetWidevinePath, DetectDevicePath, remove .env file reading)
2. **Task 1: Add FFmpeg check + flag wiring in main.go** — `5c63824` (feat: add checkFFmpeg(), Widevine device path resolution, SetWidevinePath call in main)
3. **Task 3: Update drm_test.go** — `6a3f0b8` (test: add DetectDevicePath tests, update existing tests for .env removal)

**Plan metadata:** `b1da89a` (docs(03): create phase 3 plans — config, output-dir, FFmpeg, Widevine, QOL fixes)
_Note: Task 2 was committed before Task 1 because Task 1's build verification depends on SetWidevinePath existing._

## Files Created/Modified

- `main.go` — Added `checkFFmpeg()`, `os/exec` and `internal/drm` imports, FFmpeg check call, Widevine device path resolution via `resolveString`, `drm.SetWidevinePath()` call before API client creation
- `internal/drm/drm.go` — Added `SetWidevinePath()`, `DevicePathFormat` type, `DetectDevicePath()`, `widevineDevicePath` package var; removed `readDotEnv()`, `envValue()`, `widevineEnvFile` constant, `bufio` import; updated `discoverWinevineDeviceConfig()` with new priority order; updated `missingWidevineDeviceError()` message
- `internal/drm/drm_test.go` — Added 7 new tests (DetectDevicePath acceptance/rejection x5, SetWidevinePath override x2), replaced `.env`-dependent tests with `t.Setenv()` equivalents, updated `resetWidevineDeviceCache` to also reset `widevineDevicePath`

## Decisions Made

- Task 1 (`main.go` changes) was committed after Task 2 (`drm.go` changes) because `main.go` needs `SetWidevinePath` to compile — the commit order was reordered to respect compilation requirements while keeping atomic commits.
- `widevineDevicePath` package var is NOT protected by a mutex — it's set once before any `GetWinevineDevice()` call (at startup), per the `sync.Once` Pitfall 3 pattern.
- The `strings` import was kept in `drm.go` even though `envValue` was removed — it's now used by `DetectDevicePath` (`HasSuffix`, `ToLower`).

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

- Test ordering: `widevineDevicePath` is a package-level global that persisted across tests. Multiple tests running after `TestSetWinevinePathOverridesEnvVars` would inherit the stale path value. Fixed by adding `resetWidevineDeviceCache(t)` to all tests that call `discoverWinevineDeviceConfig`, which resets `widevineDevicePath` alongside the other globals.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- FFmpeg validation ready — users get immediate feedback if ffmpeg is missing
- Widevine device path fully configurable via `--widevine-device` flag, env vars, or config file
- Next plan 03-04 (QOL fixes — regex sanitize, URL validation, parseLangs) can build on this foundation

---

*Phase: 03-usability-configuration-validation*
*Completed: 2026-07-09*