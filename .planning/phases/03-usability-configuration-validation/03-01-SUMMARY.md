---
phase: 03-usability-configuration-validation
plan: 01
subsystem: config
tags: [config, env-vars, xdg, json, go-stdlib]

requires:
  - phase: 01-foundation-error-handling-http-memory
    provides: error return patterns, stdlib conventions
  - phase: 02-performance-caching-parallelism
    provides: internal package structure, test helpers
provides:
  - internal/config package with Config struct, Load/Save/Merge/ConfigDir/WriteSkeleton
  - CRUNCHYROLL_ETP_RT env var support for etp_rt cookie
  - config file ~/.config/animeheaven/config.json with XDG discovery
  - CLI flag > env var > config file > default precedence hierarchy
  - Auto-generated config skeleton on first run
affects:
  - 03-02 (will wire outputDir into download chain)
  - 03-03 (will wire widevineDev into drm package)

tech-stack:
  added: []
  patterns:
    - Pointer-field Config struct for explicit-only JSON overrides
    - flag.Visit() for explicit CLI flag detection
    - XDG_CONFIG_HOME first with os.UserConfigDir() fallback
    - Package-level resolveString/resolveEtpRt functions for testability

key-files:
  created:
    - internal/config/config.go
    - internal/config/config_test.go
  modified:
    - main.go
    - main_test.go

key-decisions:
  - "Config struct uses *string/*int pointer fields with omitempty for explicit-only override (D-05)"
  - "Config discovery: $XDG_CONFIG_HOME/animeheaven first, then os.UserConfigDir()/animeheaven (D-03)"
  - "resolveEtpRt and resolveString extracted as package-level functions for direct unit testability"
  - "Missing config file auto-generates skeleton with defaults; invalid JSON warns via stderr (D-04)"
  - "flag.Visit() used for explicit-set detection per Go stdlib idiom"

patterns-established:
  - "Config pointer field pattern: use *string/*int with json:\",omitempty\" for JSON with explicit-only overrides"
  - "flag.Visit() map: build explicitFlags map after flag.Parse() for precedence resolution"
  - "Precedence resolution: package-level functions taking explicitFlags map + sources for testability"

requirements-completed: [USAB-01, USAB-02]

coverage:
  - id: D1
    description: "Config struct with pointer fields, Load/Save/Merge/ConfigDir/WriteSkeleton"
    requirement: USAB-02
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestLoadValidFile"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestLoadFileNotFound"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestLoadInvalidJSON"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestWriteSkeleton"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestMergeOverlayFields"
        status: pass
      - kind: unit
        ref: "internal/config/config_test.go#TestConfigDirUsesXdgConfigHome"
        status: pass
    human_judgment: false
  - id: D2
    description: "CRUNCHYROLL_ETP_RT env var support with precedence resolution"
    requirement: USAB-01
    verification:
      - kind: unit
        ref: "main_test.go#TestResolveEtpRtPrefersFlag"
        status: pass
      - kind: unit
        ref: "main_test.go#TestResolveEtpRtFallsBackToEnv"
        status: pass
      - kind: unit
        ref: "main_test.go#TestResolveEtpRtPrefersFlagOverEnv"
        status: pass
      - kind: unit
        ref: "main_test.go#TestResolveEtpRtFallsBackToConfig"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-07-09
status: complete
---

# Phase 3 Plan 1: Configuration Infrastructure and Env Var Support Summary

**Persistent JSON config at `~/.config/animeheaven/config.json` with pointer-field explicit-only overrides, XDG config discovery, CRUNCHYROLL_ETP_RT env var, and CLI flag > env var > config file > default precedence hierarchy**

## Performance

- **Duration:** 8 min
- **Started:** 2026-07-09T10:47:18Z
- **Completed:** 2026-07-09T10:55:14Z
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments

- Created `internal/config/config.go` with `Config` struct (8 pointer fields), `ConfigDir()`, `ConfigPath()`, `Load()`, `WriteSkeleton()`, and `Merge()`
- Created `internal/config/config_test.go` with 12 table-driven unit tests covering all operations
- Wired config loading, `CRUNCHYROLL_ETP_RT` env var support, and precedence hierarchy into `main.go`
- `--output-dir` and `--widevine-device` flags defined (stubbed, to be wired in Plans 3.2/3.3)
- Config skeleton auto-generated on missing file; invalid JSON warns via stderr without blocking startup
- `resolveEtpRt` and `resolveString` extracted as package-level functions for direct unit testing

## Task Commits

Each task was committed atomically:

1. **Task 1: Create internal/config/config.go** - `fb3dab4` (feat)
2. **Task 2: Create internal/config/config_test.go** - `5df156d` (test)
3. **Task 3: Wire config resolution into main.go** - `7386e9e` (feat)

**Plan metadata:** pending final commit

## Files Created/Modified

- `internal/config/config.go` - Config struct, Load, Save, Merge, ConfigDir, WriteSkeleton
- `internal/config/config_test.go` - 12 unit tests for config operations
- `main.go` - Updated with config loading, CRUNCHYROLL_ETP_RT, precedence resolution, new flags
- `main_test.go` - Added 5 tests for resolveEtpRt precedence

## Decisions Made

- Used `*string`/`*int` pointer fields with `json:",omitempty"` for explicit-only config override (matches D-05)
- Extracted `resolveEtpRt` and `resolveString` as package-level functions rather than closures inside `main()` to enable direct unit testing
- Config path resolution errors gracefully degrade (print warning, use defaults) per D-04
- Used `flag.Visit()` for explicit flag detection per Go stdlib recommendation

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- `os/exec` and `path/filepath` imports requested in plan but deferred to Plans 3.2/3.3 (not used in this plan)
- Initial `cfg` pointer initialization was nil on ConfigPath error, fixed to always initialize as `&config.Config{}`

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Config infrastructure ready for Plans 3.2 (output-dir wiring) and 3.3 (widevine-device path wiring)
- Precedence hierarchy functions (resolveString, resolveEtpRt) ready for use by all downstream plans
- Next plan: 03-02 — Output dir flag + batch URL validation

## Self-Check: PASSED

- internal/config/config.go: FOUND ✓
- internal/config/config_test.go: FOUND ✓
- main.go: FOUND ✓
- main_test.go: FOUND ✓
- Commits: 3 task commits present (2 feat, 1 test) ✓
- `go build .`: PASS ✓
- `go test ./internal/config/...`: PASS ✓
- `go test . -run TestResolve`: PASS ✓

---

*Phase: 03-usability-configuration-validation*
*Completed: 2026-07-09*
